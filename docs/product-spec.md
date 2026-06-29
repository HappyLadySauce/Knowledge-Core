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

核心思路：**文件系统即存储，Markdown 即数据格式。**

- 数据存放于用户家目录 `~/knowledge-core/`
- 笔记、文章以 Markdown 文件存储
- 分类采用目录结构（粗分类，一级/二级）
- 真正的知识关系通过 frontmatter 中的 tags 和双向链接 `[[]]` 表达
- 系统维护 SQLite 索引数据库，加速 tag/category/搜索查询

## 技术栈

| 层 | 选型 | 理由 |
|---|---|---|
| 后端 | Go + Gin/Fiber | 轻量、高性能、文件操作方便 |
| 前端 | React + Next.js + TypeScript | Markdown 生态成熟、SSR/SSG 支持 |
| 样式 | Tailwind CSS | 快速迭代，与 Next.js 搭配好 |
| Markdown 渲染 | remark + rehype 插件链 | 可扩展的 AST 转换 |
| 索引数据库 | SQLite | 本地零配置，存储 tag/category/search 索引 |
| AI 接口 | OpenAI SDK / Anthropic SDK | 导入分析，后续可替换 |

## 目录结构

```
~/knowledge-core/
├── notes/                    # 笔记正文（按粗分类组织）
│   ├── tech/
│   │   ├── ai/
│   │   │   └── 深度学习入门.md
│   │   └── frontend/
│   │       └── React Hooks 总结.md
│   ├── life/
│   │   └── 2026年目标.md
│   └── reading/
│       └── 思考快与慢.md
├── tags/                     # 可选：特殊 tag 资源
├── images/                   # 图片附件统一存放
├── attachments/              # 其他附件
└── _templates/               # 笔记模板

~/.knowledge-core/
└── index.db                  # SQLite 索引（不放在笔记目录中）
```

## Frontmatter Schema

```yaml
---
title: "深度学习入门笔记"
created: 2026-06-28T10:00:00+08:00
updated: 2026-06-28T14:30:00+08:00
tags: [ai, deep-learning, learning]
category: tech/ai
source: import              # import | manual | agent
summary: "..."
confidence: 0.3              # AI 分析置信度（低于阈值提示用户确认）
---
```

字段说明：

- `title` — 笔记标题（AI 导入时自动生成，用户可修改）
- `created` — 创建时间
- `updated` — 最后更新时间
- `tags` — 标签列表（AI 自动生成 + 用户手动补充）
- `category` — 所属目录分类
- `source` — 来源标识：`import`（AI 导入）、`manual`（手动创建）、`agent`（Agent 生成）
- `summary` — AI 生成的摘要
- `confidence` — AI 分析置信度，低于阈值时提示用户确认后再写入

## AI 导入管线

当用户导入 Markdown 文件时：

1. 读取文件内容
2. 调用 AI 接口分析：生成标题、分类建议、标签、摘要
3. 返回分析结果，若 `confidence` 低于阈值则让用户确认
4. 写入 frontmatter 到文件头部
5. 移动文件到对应 category 目录
6. 更新 SQLite 索引

### 边界情况处理

- **已有 frontmatter 的文件**：合并处理，不覆盖用户已手动修改的字段
- **用户不满意 AI 分类/标签**：支持手动修改，修改后的数据反馈到索引，AI 下次学习用户偏好
- **大量导入**：异步队列处理，前端展示进度
- **AI 接口不可用**：本地降级方案——使用基础规则（按文件名分词提取标题，无 AI 标签/摘要）

## 用户系统

### 用户角色

| 角色 | 权限 | 默认账号 |
|---|---|---|
| 管理员 | 全部权限：笔记 CRUD、导入/导出、用户管理、系统设置、前台内容发布 | 首次启动自动创建 |
| 普通用户 | 前台浏览已发布文章、注册/登录、个人资料修改、评论（v2） | 注册创建 |

### 用户模型

```go
type User struct {
    ID        uint      `json:"id"`
    Username  string    `json:"username" gorm:"uniqueIndex"`
    Email     string    `json:"email" gorm:"uniqueIndex"`
    Password  string    `json:"-"`           // bcrypt 哈希
    Role      string    `json:"role"`         // "admin" | "user"
    Avatar    string    `json:"avatar"`       // 头像 URL
    Bio       string    `json:"bio"`          // 个人简介
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 认证方式

- JWT Token（Access Token + Refresh Token）
- Access Token 有效期 2 小时，Refresh Token 7 天
- Token 存储在 httpOnly Cookie 中
- 后端中间件校验 Role 字段进行权限控制

### API 路由

```
POST   /api/auth/register      # 注册（公开）
POST   /api/auth/login         # 登录（公开）
POST   /api/auth/refresh       # 刷新 Token（公开）
POST   /api/auth/logout        # 登出

GET    /api/users/me            # 当前用户信息（需登录）
PUT    /api/users/me            # 更新个人资料（需登录）
PUT    /api/users/me/password   # 修改密码（需登录）

GET    /api/users               # 用户列表（仅 admin）
PUT    /api/users/:id/role      # 修改用户角色（仅 admin）
DELETE /api/users/:id           # 删除用户（仅 admin）
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
    ID        uint      `json:"id"`
    ArticleID uint      `json:"article_id" gorm:"index"`
    UserID    uint      `json:"user_id" gorm:"index"`
    ParentID  uint      `json:"parent_id"`       // 0 = 顶级评论
    Content   string    `json:"content"`          // Markdown 格式
    Status    string    `json:"status"`           // "pending" | "approved" | "rejected"
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`

    User    User    `json:"user"    gorm:"preload"`
    Article Article `json:"article" gorm:"preload"`
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
GET    /api/articles/:id/comments          # 获取文章评论（仅 approved，公开）
POST   /api/articles/:id/comments          # 发表评论（需登录）
DELETE /api/comments/:id                  # 删除评论（本人或 admin）

GET    /api/admin/comments?status=pending  # 待审核评论列表（仅 admin）
PUT    /api/admin/comments/:id/approve      # 审核通过（仅 admin）
PUT    /api/admin/comments/:id/reject       # 审核拒绝（仅 admin）
DELETE /api/admin/comments/:id             # 删除评论（仅 admin）
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
    ID       uint   `json:"id"`
    Name     string `json:"name" gorm:"uniqueIndex"`
    Slug     string `json:"slug" gorm:"uniqueIndex"` // URL 友好标识
    ParentID uint   `json:"parent_id"`               // 支持层级分类
    Sort     int    `json:"sort"`                    // 排序权重
}

type Tag struct {
    ID   uint   `json:"id"`
    Name string `json:"name" gorm:"uniqueIndex"`
    Slug string `json:"slug" gorm:"uniqueIndex"`
}
```

### 管理功能

| 操作 | 分类 | 标签 |
|---|---|---|
| 创建 | 仅管理员 | 仅管理员 |
| 编辑 | 仅管理员 | 仅管理员 |
| 删除 | 仅管理员（级联解除关联） | 仅管理员 |
| 合并 | 仅管理员（将 A 合入 B） | 仅管理员 |
| 重命名 | 仅管理员（自动更新 slug） | 仅管理员 |

### API 路由

```
# 分类
GET    /api/admin/categories          # 分类列表（树形）
POST   /api/admin/categories          # 创建分类
PUT    /api/admin/categories/:id      # 更新分类
DELETE /api/admin/categories/:id      # 删除分类
PUT    /api/admin/categories/merge    # 合并分类

# 标签
GET    /api/admin/tags                # 标签列表
POST   /api/admin/tags                # 创建标签
PUT    /api/admin/tags/:id            # 更新标签
DELETE /api/admin/tags/:id            # 删除标签
PUT    /api/admin/tags/merge          # 合并标签
```

## 文章编辑器

### 编辑器功能

- 所见即所得 Markdown 编辑器（左右分栏：左侧编辑，右侧实时预览）
- 支持 frontmatter 编辑（分类、标签、标题、发布状态）
- 工具栏：加粗、斜体、标题(H1-H3)、代码块、链接、图片、引用、列表、表格
- 自动保存草稿（每 30 秒）
- AI 辅助：AI 生成摘要、AI 推荐标签、AI 续写（调用后台设置中的 AI 服务）
- 发布/草稿状态切换

### 数据模型扩展

```go
// Article 新增字段
Status   string `json:"status"`    // "draft" | "published"
AuthorID uint   `json:"author_id"` // 作者（管理员 ID）
```

### API 路由

```
POST   /api/admin/articles              # 创建文章（草稿）
PUT    /api/admin/articles/:id          # 更新文章
PUT    /api/admin/articles/:id/publish  # 发布文章
PUT    /api/admin/articles/:id/unpublish # 取消发布
DELETE /api/admin/articles/:id          # 删除文章
POST   /api/admin/articles/:id/ai-summary  # AI 生成摘要
POST   /api/admin/articles/:id/ai-tags     # AI 推荐标签
```

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

1. Markdown 文件导入 + AI 自动分析（分类、标签、摘要、标题）
2. SQLite 索引（tag → 文件、category → 文件、全文搜索）
3. Web 端笔记列表浏览 + 搜索 + 阅读
4. 基于 frontmatter 的笔记详情渲染
5. 用户注册/登录（JWT 认证）
6. 管理员后台权限控制
7. 前台用户个人中心
8. 文章评论系统（前台浏览 + 管理员审核）
9. 后台设置（站点配置、AI API、SEO）
10. 数据看板（访问量、文章趋势、用户增长）
11. 标签与分类管理（后台 CRUD + 合并）
12. 文章编辑器（Markdown 分栏编辑 + AI 辅助 + 草稿/发布）
13. 导出功能（Markdown/PDF/JSON/ZIP）

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

## 设计原则

1. **用户数据主权** — 所有数据存放在用户本地文件系统，用户随时可用任何工具访问
2. **渐进式 AI** — AI 辅助但不强制，所有 AI 生成内容用户可审阅、可修改、可撤销
3. **文件系统即真相** — SQLite 索引是缓存，md 文件是数据源，索引可随时从文件系统重建
4. **零迁移成本** — 标准 Markdown 格式，可随时迁移到任何支持 Markdown 的工具
