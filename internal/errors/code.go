package errors

import (
	"net/http"
)

const (
	MessageOK              = "ok"
	MessageInvalidRequest  = "invalid_request"
	MessageUnauthorized    = "unauthorized"
	MessageForbidden       = "forbidden"
	MessageNotFound        = "not_found"
	MessageConflict        = "conflict"
	MessageInternalError   = "internal_error"
	CodeInvalidRequest     = "invalid_request"
	CodeInvalidCredentials = "invalid_credentials"
	CodeUserDisabled       = "user_disabled"
	CodeConflict           = "conflict"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeInvalidToken       = "invalid_token"
	CodeInternalError      = "internal_error"
)

var (
	InvalidRequest     = New(CodeInvalidRequest, http.StatusBadRequest, MessageInvalidRequest)
	InvalidCredentials = New(CodeInvalidCredentials, http.StatusUnauthorized, MessageUnauthorized)
	UserDisabled       = New(CodeUserDisabled, http.StatusUnauthorized, MessageUnauthorized)
	Conflict           = New(CodeConflict, http.StatusConflict, MessageConflict)
	Forbidden          = New(CodeForbidden, http.StatusForbidden, MessageForbidden)
	NotFound           = New(CodeNotFound, http.StatusNotFound, MessageNotFound)
	InvalidToken       = New(CodeInvalidToken, http.StatusUnauthorized, MessageUnauthorized)
	InternalError      = New(CodeInternalError, http.StatusInternalServerError, MessageInternalError)
)
