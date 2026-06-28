package errors

import (
	"errors"
)

// AppError carries a stable project error code and HTTP mapping.
// AppError 携带稳定的项目错误码与 HTTP 映射。
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	cause      error
}

// New creates a reusable application error sentinel.
// New 创建可复用的应用错误哨兵值。
func New(code string, status int, message string) *AppError {
	return &AppError{Code: code, HTTPStatus: status, Message: message}
}

// Wrap attaches a cause to an application error.
// Wrap 为应用错误附加底层原因。
func Wrap(appErr *AppError, cause error) error {
	if cause == nil {
		return appErr
	}
	return &AppError{
		Code:       appErr.Code,
		Message:    appErr.Message,
		HTTPStatus: appErr.HTTPStatus,
		cause:      cause,
	}
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return e.Code + ": " + e.cause.Error()
	}
	return e.Code
}

func (e *AppError) Unwrap() error {
	return e.cause
}

func (e *AppError) Is(target error) bool {
	targetErr, ok := target.(*AppError)
	return ok && targetErr.Code == e.Code
}

// From maps any error to an AppError; unknown errors become InternalError.
// From 将任意错误映射为 AppError；未知错误会变为 InternalError。
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Wrap(InternalError, err).(*AppError)
}

// Is reports whether err matches target.
// Is 判断 err 是否匹配 target。
func Is(err error, target *AppError) bool {
	return errors.Is(err, target)
}
