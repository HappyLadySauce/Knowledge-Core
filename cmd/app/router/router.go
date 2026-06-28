package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	router *gin.Engine
	v1     *gin.RouterGroup
)

func init() {
	router = gin.Default()

	// Set up HTTP routes and API groups.
	// 配置 HTTP 路由与 API 分组。
	v1 = router.Group("/api/v1")

	router.GET("/livez", func(c *gin.Context) {
		c.String(200, "livez")
	})
	router.GET("/readyz", func(c *gin.Context) {
		c.String(200, "readyz")
	})

	// Register Swagger UI routes.
	// 注册 Swagger UI 路由。
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// V1 returns the router group for /api/v1 (control-plane style resource endpoints).
// V1 返回 /api/v1 的路由分组（面向控制面风格的资源端点）。
func V1() *gin.RouterGroup {
	return v1
}

// Router returns the main Gin engine instance.
// Router 返回主 Gin 引擎实例。
func Router() *gin.Engine {
	return router
}

// ConfigureTrustedProxies sets trusted proxy CIDRs or IPs for ClientIP; empty slice clears custom trust.
// ConfigureTrustedProxies 设置用于解析 ClientIP 的可信代理 CIDR/IP；空切片表示不信任代理。
func ConfigureTrustedProxies(trusted []string) error {
	if len(trusted) == 0 {
		trusted = nil
	}
	return router.SetTrustedProxies(trusted)
}
