package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/component-base/cli"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app"
	// _ "github.com/HappyLadySauce/Knowledge-Core/api/swagger/docs" // Register embedded Swagger spec for gin-swagger UI.
)

// @title       Knowledge Core HTTP API
// @version     1.0
// @description REST API for Knowledge Core (v1). JSON envelope: code (HTTP-style), message, data. 中文：Knowledge Core REST API（v1）；响应为 code、message、data 的 JSON 包装。
// @BasePath    /
// @schemes     http https

const (
	basename = "Knowledge-Core"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := app.NewAPICommand(ctx, basename)
	code := cli.Run(cmd)
	os.Exit(code)
}
