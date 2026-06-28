package main

import (
	"context"
	"os"

	"k8s.io/component-base/cli"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app"

	// _ "github.com/HappyLadySauce/Knowledge-Core/api/swagger/docs" // Register embedded Swagger spec for gin-swagger UI.
)

// @title       Beehive Blog HTTP API
// @version     1.0
// @description REST API for Beehive Blog (v1). JSON envelope: code (HTTP-style), message, data. 中文：Beehive Blog REST API（v1）；响应为 code、message、data 的 JSON 包装。
// @BasePath    /
// @schemes     http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Send header Authorization with value "Bearer " plus the JWT access token. 中文：请求头 Authorization，值为 Bearer、空格与 access token 拼接。

const (
	basename = "Knowledge-Core"
)

func main() {
	ctx := context.TODO()
	cmd := app.NewAPICommand(ctx, basename)
	code := cli.Run(cmd)
	os.Exit(code)
}
