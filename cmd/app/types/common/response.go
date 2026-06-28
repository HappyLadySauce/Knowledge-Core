package common

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

// Response is the common HTTP response envelope.
// Response 是通用 HTTP 响应包装结构。
type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// OK writes a 200 response envelope.
// OK 写入 200 响应包装。
func OK[T any](c *gin.Context, data T) {
	JSON(c, http.StatusOK, apperrors.MessageOK, data)
}

// Created writes a 201 response envelope.
// Created 写入 201 响应包装。
func Created[T any](c *gin.Context, data T) {
	JSON(c, http.StatusCreated, apperrors.MessageOK, data)
}

// Error writes an error response envelope.
// Error 写入错误响应包装。
func Error(c *gin.Context, err error) {
	appErr := apperrors.From(err)
	c.JSON(appErr.HTTPStatus, Response[any]{
		Code:    appErr.HTTPStatus,
		Message: appErr.Message,
		Data:    nil,
	})
}

// JSON writes an arbitrary status response envelope.
// JSON 写入指定状态码的响应包装。
func JSON[T any](c *gin.Context, status int, message string, data T) {
	c.JSON(status, Response[T]{
		Code:    status,
		Message: message,
		Data:    data,
	})
}
