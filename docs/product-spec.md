# Knowledge Core - 产品规格文档

> 一个懂你过去、整理你现在、陪你走向未来的个人 AI 成长操作系统。

## 产品定位

个人知识中台 + 灵魂伴侣 Agent。用户将学习笔记、日记、照片、经历、项目记录等全部沉淀到系统中，系统通过 AI 理解、整理、分析这些数据，形成真正懂用户的个人 Agent。

## 核心价值

1. **记录个人数据** — 学习、生活、经历、照片、想法、目标统一保存，形成个人长期数据资产
2. **理解用户状态** — 分析学过什么、掌握程度、关注方向、情绪变化、反复出现的问题
3. **陪伴并引导成长** — 像长期伙伴一样理解用户，帮助复盘过去、整理现在、规划未来

## 与普通笔记软件的本质差异

| 普通笔记软件 | Knowledge Core |
|---|---|
| 我存了什么？ | 我是谁？我经历了什么？我学会了什么？我接下来该走向哪里？ |

## 系统架构（五层）

```
个人数据层
  笔记 / 日记 / 照片 / 文件 / 学习记录 / 项目经历
    ↓
结构化记忆层
  标签 / 时间 / 主题 / 人物 / 情绪 / 目标 / 知识点
    ↓
知识分析层
  学习掌握度 / 知识图谱 / 情绪趋势 / 目标进度 / 行为模式
    ↓
AI Agent 层
  聊天陪伴 / 学习教练 / 生活复盘 / 项目助手 / 主动建议
    ↓
展示与操作层
  Web 前端 / 桌面客户端 / 文件管理 / 任务系统
```

## 产品形态

- **Web 前端**：知识库浏览、时间线、学习分析、AI 总结、数据看板
- **桌面客户端**（v2）：本地文件管理、笔记导入、照片整理、快捷记录、Agent 操作电脑

---

# 第一阶段：用户知识管理

## 技术路线

核心思路：**PostgreSQL 是文档最终数据源，Markdown 仅作为导入/导出格式。**

- 文档元数据、正文块、协作操作日志、发布快照、分类和标签统一存储在 PostgreSQL。
- 正文采用块结构 JSON：不同块可以并发编辑，同一块通过版本校验返回冲突快照。
- `document_ops` 保存操作日志，用于幂等提交、审计和断线后增量同步。
- `document_revisions` 保存发布快照，前台公开详情只读取已发布 revision，不读取编辑态 blocks。
- PostgreSQL `tsvector + GIN` 负责全文搜索，搜索文本由标题、摘要、分类、标签和正文块聚合。
- 文件系统不参与在线编辑主链路，后续只用于 Markdown 导入/导出临时文件。

## 技术栈

| 层 | 选型 | 理由 |
|---|---|---|
| 后端 | Go + Gin | 轻量、高性能，当前代码采用 Gin 路由 |
| 数据访问 | `database/sql` + `pgx` stdlib | 保留显式 SQL、事务和行级锁控制 |
| 前端 | React + Next.js + TypeScript | Markdown 生态成熟、SSR/SSG 支持 |
| 样式 | Tailwind CSS | 快速迭代，与 Next.js 搭配好 |
| Markdown 渲染 | remark + rehype 插件链 | 可扩展的 AST 转换 |
| 主数据库 | PostgreSQL 16 | 支持事务、行级锁、JSONB、全文搜索和协作数据一致性 |
| AI 接口 | OpenAI SDK / Anthropic SDK | 导入分析，后续可替换 |

## 本地开发运行

```powershell
copy .env.example .env
docker compose up -d postgres redis
make migrate
make run
```

默认数据库连接：

```text
postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core?sslmode=disable
```

测试数据库：

```text
postgres://knowledge_core:knowledge_core@localhost:5432/knowledge_core_test?sslmode=disable
```

默认 Redis 连接：

```text
redis://localhost:6379/0
```

JWT 密钥为空或使用示例弱默认值时，开发环境会生成本次会话随机密钥；生产环境必须设置唯一的 `KNOWLEDGE_CORE_JWT_SECRET`，否则重启后旧 token 会失效。

## 文档存储模型

### documents

- 保存文档元数据：标题、摘要、slug、分类、作者、发布状态、当前版本、发布时间。
- `status` 使用 `draft` / `published`。
- `current_version` 是编辑态文档版本，每个成功 op 递增一次。
- `search_vector` 是 PostgreSQL generated tsvector，用于全文搜索。

### document_blocks

- 保存正文块：`block_id, document_id, parent_id, position_key, type, content_json, text_content, version, updated_by, updated_at`。
- `content_json` 使用 JSONB，当前 MVP 以 paragraph 块为主。
- 同一文档内不同块可以并发编辑；同一块版本不匹配返回冲突。

### document_ops

- 保存协作操作日志：`op_id, document_id, actor_id, base_document_version, block_id, op_type, payload_json, document_version, block_version, created_at`。
- `op_id` 全局唯一，用于幂等提交。重复提交同一 `op_id` 返回原始 ack，不重复修改正文。

### document_revisions

- 保存发布快照或手动快照。
- 前台公开详情只读取最新已发布 revision，继续编辑草稿不会影响公开内容。

## Markdown 导入/导出

Markdown 不是在线编辑的最终存储格式。

- 导入：`Markdown -> blocks_v1 JSONB`。
- 导出：`blocks/revision -> Markdown`。
- frontmatter 可作为导入/导出的边界格式，但不作为在线编辑主链路。

## 用户系统

### 用户角色

| 角色 | 权限 | 默认账号 |
|---|---|---|
| 管理员 | 全部权限：笔记 CRUD、导入/导出、用户管理、系统设置、前台内容发布 | 首次启动自动创建 |
| 普通用户 | 前台浏览已发布文章、注册/登录、个人资料修改、评论（v2） | 注册创建 |

### 用户模型

```go
type User struct {
    ID        int64     `json:"id"`
    Username  string    `json:"username"`
    Email     string    `json:"email"`
    Password  string    `json:"-"`           // bcrypt 哈希
    Role      string    `json:"role"`         // "admin" | "user"
    Status    string    `json:"status"`       // "active" | "disabled"
    TokenVersion int64  `json:"token_version"`
    Avatar    string    `json:"avatar"`       // 头像 URL
    Bio       string    `json:"bio"`          // 个人简介
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 认证方式

- Access Token 使用 JWT，后端中间件校验 `role` 与 `token_version`。
- Refresh Token 明文只返回给客户端，服务端只保存 SHA-256 hash。
- Redis 保存活跃 refresh token 会话元数据，PostgreSQL `refresh_tokens` 保留审计与 Redis 故障降级。
- Refresh 时即使 Redis 命中也会强校验 PostgreSQL 中的撤销、过期、用户状态和 `token_version`。
- 修改密码、禁用用户、角色/状态变化会递增 `token_version` 并撤销该用户全部 refresh token。

### API 路由

```
POST   /api/v1/auth/register      # 注册（公开）
POST   /api/v1/auth/login         # 登录（公开）
POST   /api/v1/auth/refresh       # 刷新 Token（公开）
POST   /api/v1/auth/logout        # 登出（需登录）

GET    /api/v1/users/me            # 当前用户信息（需登录）
PUT    /api/v1/users/me            # 更新个人资料（需登录）
PUT    /api/v1/users/me/password   # 修改密码（需登录）

GET    /api/v1/admin/users         # 用户列表（仅 admin）
GET    /api/v1/admin/users/:id     # 用户详情（仅 admin）
PATCH  /api/v1/admin/users/:id     # 修改用户资料/角色/状态（仅 admin）
DELETE /api/v1/admin/users/:id     # 禁用用户（仅 admin）
PUT    /api/v1/admin/users/:id/password # 重置密码（仅 admin）
```

### 前台与后台的用户体验区分

- **前台**：注册/登录后可浏览已发布文章，个人中心可修改头像和简介
- **后台**：仅管理员可访问（`/admin`），使用管理员账号登录
- 前台登录和后台登录共用同一套认证系统，通过 Role 区分权限

### 安全策略

- 密码 bcrypt 加密存储
- 注册时邮箱验证（v2，MVP 阶段可跳过）
- 登录失败 5 次后锁定 15 分钟
- JWT 黑名单机制支持强制登出

## 评论系统

### 数据模型

```go
type Comment struct {
    ID         int64     `json:"id"`
    DocumentID int64     `json:"document_id"`
    UserID     int64     `json:"user_id"`
    ParentID   int64     `json:"parent_id"`       // 0 = 顶级评论
    Content    string    `json:"content"`         // Markdown 格式
    Status     string    `json:"status"`          // "pending" | "approved" | "rejected"
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

### 评论规则

| 规则 | 说明 |
|---|---|
| 发评论 | 需登录，Markdown 格式，限 2000 字 |
| 评论状态 | 新评论默认 `pending`，管理员审核后变为 `approved` |
| 审核机制 | 管理员可在后台批量审核/拒绝，前端只显示 `approved` 评论 |
| 作者高亮 | 文章作者（管理员）的评论带特殊标识 |
| 删除评论 | 管理员可删除任何评论，用户只能删除自己的评论 |
| v2 扩展 | 嵌套回复（ParentID）、点赞、举报 |

### API 路由

```
GET    /api/v1/documents/:id/comments          # 获取评论（仅 approved，公开）
POST   /api/v1/documents/:id/comments          # 发表评论（需登录）
DELETE /api/v1/comments/:id                    # 删除评论（本人或 admin）

GET    /api/v1/admin/comments?status=pending   # 待审核评论列表（仅 admin）
PUT    /api/v1/admin/comments/:id/approve      # 审核通过（仅 admin）
PUT    /api/v1/admin/comments/:id/reject       # 审核拒绝（仅 admin）
DELETE /api/v1/admin/comments/:id              # 删除评论（仅 admin）
```

### 前端展示

- 文章阅读页底部显示评论列表（按时间倒序）
- 每条评论显示：头像、用户名、时间、内容、操作
- 管理员评论显示“作者”标签
- 未登录用户显示登录提示，隐藏输入框

## 后台设置

### 设置项

```go
type SiteSettings struct {
    SiteName        string `json:"site_name"`         // 站点名称
    SiteDescription string `json:"site_description"` // 站点描述
    SiteURL         string `json:"site_url"`          // 站点地址
    AdminEmail      string `json:"admin_email"`      // 管理员邮箱
    AllowRegister   bool   `json:"allow_register"`   // 是否允许注册
    CommentModerate bool   `json:"comment_moderate"`  // 是否开启评论审核
    PostsPerPage    int    `json:"posts_per_page"`    // 每页文章数
    Theme           string `json:"theme"`             // 主题: "light" | "dark" | "auto"
    AIProvider      string `json:"ai_provider"`       // AI 服务商: "openai" | "anthropic"
    AIModel         string `json:"ai_model"`          // 模型名称
    APIKey          string `json:"-"`                 // API Key（前端掩码显示）
    APIBaseURL      string `json:"api_base_url"`     // 自定义 API 地址
}
```

### API 路由

```
GET  /api/admin/settings           # 获取设置（仅 admin，API Key 脱敏）
PUT  /api/admin/settings           # 更新设置（仅 admin）
POST /api/admin/settings/test-ai   # 测试 AI 连接（仅 admin）
```

### 设置页面分区

| 分区 | 内容 |
|---|---|
| 基本设置 | 站点名称、描述、URL、管理员邮箱、开关注册 |
| 评论设置 | 是否开启审核、每页评论数 |
| 外观设置 | 每页文章数、主题选择 |
| AI 设置 | 服务商选择、模型名称、API Key（掩码）、API 地址、测试连接按钮 |

## 数据看板

### 统计指标

| 指标 | 说明 | 时间维度 |
|---|---|---|
| 总访问量 | PV/UV | 今日 / 7日 / 30日 |
| 文章总数 | 已发布文章 | 累计 |
| 评论总数 | 已通过评论 | 累计 / 今日新增 |
| 用户总数 | 注册用户 | 累计 / 今日新增 |
| 热门文章 Top 10 | 按浏览量排序 | 7日 / 30日 / 全部 |
| 文章趋势 | 每日新增文章折线图 | 近 30 天 |
| 访问趋势 | 每日 PV 折线图 | 近 30 天 |
| 用户增长 | 每日新注册用户 | 近 30 天 |

### API 路由

```
GET /api/admin/dashboard/overview   # 总览统计
GET /api/admin/dashboard/trends    # 趋势数据（articles, visits, users, ?days=30）
GET /api/admin/dashboard/top-articles?limit=10&period=7d  # 热门文章
```

## 标签与分类管理

### 数据模型

```go
type Category struct {
    ID       int64  `json:"id"`
    Name     string `json:"name"`
    Slug     string `json:"slug"`      // URL 友好标识
    Path     string `json:"path"`      // 层级路径，如 tech/ai
    ParentID int64  `json:"parent_id"` // 支持层级分类
    Sort     int    `json:"sort"`                    // 排序权重
}

type Tag struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
    Slug string `json:"slug"`
}
```

### 管理功能

| 操作 | 分类 | 标签 |
|---|---|---|
| 创建 | 仅管理员 | 仅管理员 |
| 编辑 | 仅管理员 | 仅管理员 |
| 删除 | 仅管理员；仅允许无子分类且无文档引用时删除 | 仅管理员；删除前必须无文档引用 |
| 合并 | 后续阶段 | 后续阶段 |
| 重命名 | 仅管理员（自动更新 slug） | 仅管理员 |

### API 路由

```
# 分类
GET    /api/v1/categories                # 公开分类列表（仅含已发布内容关联）
GET    /api/v1/admin/categories          # 分类列表（admin）
POST   /api/v1/admin/categories          # 创建分类
PATCH  /api/v1/admin/categories/:id      # 更新分类
DELETE /api/v1/admin/categories/:id      # 删除分类

# 标签
GET    /api/v1/tags                      # 公开标签列表（仅含已发布内容关联）
GET    /api/v1/admin/tags                # 标签列表（admin）
POST   /api/v1/admin/tags                # 创建标签
PATCH  /api/v1/admin/tags/:id            # 更新标签
DELETE /api/v1/admin/tags/:id            # 删除标签
```

## 文章编辑器

### 编辑器功能

- 块级文档编辑器，内部数据格式为 blocks JSON。
- 支持 Markdown 导入/导出，但在线编辑不直接写 Markdown 文件。
- 支持 metadata 编辑（分类、标签、标题、摘要、发布状态）
- 工具栏：加粗、斜体、标题(H1-H3)、代码块、链接、图片、引用、列表、表格
- 自动保存通过 `POST /api/v1/admin/documents/:id/ops` 或 WebSocket `op` 消息提交。
- 多人实时协作使用 WebSocket：同文档不同块可以并发编辑，同一块版本冲突返回 conflict。
- 发布/草稿状态通过 `PATCH /api/v1/admin/documents/:id` 修改 `status`。
- AI 辅助（摘要、标签、续写）属于后续阶段。

### 数据模型扩展

```go
type Document struct {
    ID             int64
    Slug           string
    Title          string
    Summary        string
    CategoryID     int64
    Status         string // "draft" | "published"
    AuthorID       int64
    CurrentVersion int64
    PublishedAt    *time.Time
}

type DocumentBlock struct {
    BlockID     string
    DocumentID   int64
    ParentID     string
    PositionKey  string
    Type         string
    ContentJSON  string
    TextContent  string
    Version      int64
}
```

### API 路由

```
GET    /api/v1/documents                         # 公开已发布文档列表
GET    /api/v1/documents/:id                     # 公开文档详情（读取 published revision）

GET    /api/v1/admin/documents                   # admin 文档列表
POST   /api/v1/admin/documents                   # 创建文档
GET    /api/v1/admin/documents/:id               # 当前编辑态详情（metadata + blocks）
PATCH  /api/v1/admin/documents/:id               # 更新 metadata、blocks 或 status
DELETE /api/v1/admin/documents/:id               # 删除文档
POST   /api/v1/admin/documents/:id/ops           # HTTP 块级操作提交
GET    /api/v1/admin/documents/:id/collab        # WebSocket 协作通道
```

WebSocket 消息类型固定为：`hello`、`snapshot`、`op`、`ack`、`conflict`、`presence`、`error`。

## 导出功能

### 支持的导出格式

| 格式 | 说明 | 批量 |
|---|---|---|
| Markdown (.md) | 原始 Markdown + frontmatter | 支持 |
| PDF | 渲染后的文章（含样式） | 单篇 |
| JSON | 结构化数据（frontmatter + content） | 支持 |
| ZIP | 批量打包为 Markdown 文件集合 | 支持 |

### API 路由

```
GET    /api/admin/export/markdown/:id    # 导出单篇 Markdown
GET    /api/admin/export/pdf/:id         # 导出单篇 PDF
GET    /api/admin/export/json/:id        # 导出单篇 JSON
POST   /api/admin/export/batch           # 批量导出（JSON body: article_ids + format）
GET    /api/admin/export/download/:task_id # 下载导出文件
```

### 导出流程

1. 用户选择文章（单篇或勾选多篇）
2. 选择导出格式
3. 后端生成文件，返回下载链接
4. 批量导出返回 ZIP 文件

## MVP 范围（第一阶段）

### 必做（最小闭环）

1. PostgreSQL 文档主存储（documents、document_blocks、document_ops、document_revisions）
2. Web 端文档列表浏览 + PostgreSQL 全文搜索 + 公开阅读
3. 基于 published revision 的公开详情渲染
4. Admin 文档 CRUD、块级 ops 提交、发布/取消发布
5. 用户注册/登录（JWT 认证）
6. 管理员后台权限控制
7. 前台用户个人中心
8. 文章评论系统（前台浏览 + 管理员审核）
9. 后台设置（站点配置、AI API、SEO）
10. 数据看板（访问量、文章趋势、用户增长）
11. 标签与分类管理（后台 CRUD）
12. 块级协作编辑器（HTTP ops + WebSocket collab）
13. 导出功能（Markdown/PDF/JSON/ZIP，后续接入）

### 暂不做（后续阶段）

- AI Agent 陪伴人格
- 学习掌握度分析
- 时间线视图
- 桌面客户端
- 主动提醒/复盘
- 知识图谱可视化
- 邮箱验证
- 嵌套回复（楼中楼）
- 评论点赞/举报
- 友链功能
- 字符级 CRDT/OT
- 分类/标签合并
- AI 摘要、AI 标签、AI 续写
- 文件系统作为在线编辑主存储

## 设计原则

1. **数据一致性优先** — 在线编辑主链路以 PostgreSQL 事务和行级锁保证一致性
2. **渐进式 AI** — AI 辅助但不强制，所有 AI 生成内容用户可审阅、可修改、可撤销
3. **块级协作优先** — 先解决多人协作的幂等、冲突、发布快照，再扩展更细粒度协同
4. **开放导入导出** — Markdown 是系统边界格式，方便迁移和备份，但不是在线编辑数据源
