package handler

import (
	"errors"

	"go.uber.org/zap"
)

var (
	ErrInvalidInputData    = errors.New("validation: input data is incomplete or incorrect")
	ErrInternalServerError = errors.New("internal server error")
	ErrUserNotAllowed      = errors.New("user not allowed to perform this action")
	ErrInvalidRequestBody  = errors.New("request body is invalid")
	ErrNotAuthorized       = errors.New("invalid email or password")
)

type BaseHandler struct {
	Logger *zap.Logger
}

func NewBaseHandler(logger *zap.Logger) *BaseHandler {
	return &BaseHandler{
		Logger: logger,
	}
}
