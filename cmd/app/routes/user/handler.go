package user

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/middleware"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/router"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/svc"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/common"
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
	internaluser "github.com/HappyLadySauce/Knowledge-Core/internal/user"
)

type Controller struct {
	service internaluser.UserService
}

func NewController(sc *svc.ServiceContext) *Controller {
	return &Controller{service: internaluser.NewService(sc.DB)}
}

// Init registers user routes.
// Init 注册用户路由。
func Init(ctx context.Context, sc *svc.ServiceContext) {
	_ = ctx
	RegisterRoutes(router.V1(), internaluser.NewService(sc.DB), sc)
}

// RegisterRoutes attaches user profile and admin user management routes.
// RegisterRoutes 挂载用户资料与 admin 用户管理路由。
func RegisterRoutes(group *gin.RouterGroup, service internaluser.UserService, sc *svc.ServiceContext) {
	controller := &Controller{service: service}
	userGroup := group.Group("/users", middleware.AuthMiddleware(sc))
	userGroup.GET("/me", controller.Me)
	userGroup.PUT("/me", controller.UpdateMe)
	userGroup.PUT("/me/password", controller.ChangePassword)

	adminGroup := group.Group("/admin", middleware.AuthMiddleware(sc), middleware.RequireAdmin())
	adminGroup.GET("/users", controller.ListUsers)
	adminGroup.GET("/users/:id", controller.GetUser)
	adminGroup.PATCH("/users/:id", controller.UpdateUser)
	adminGroup.DELETE("/users/:id", controller.DeleteUser)
	adminGroup.PUT("/users/:id/password", controller.ResetPassword)
}

func (h *Controller) Me(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	currentUser, err := h.service.GetMe(c.Request.Context(), actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toUserResponse(currentUser))
}

func (h *Controller) UpdateMe(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	var req v1.UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	currentUser, err := h.service.UpdateMe(c.Request.Context(), actor, internaluser.UpdateProfileCommand{
		Username: req.Username,
		Email:    req.Email,
		Avatar:   req.Avatar,
		Bio:      req.Bio,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toUserResponse(currentUser))
}

func (h *Controller) ChangePassword(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	var req v1.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), actor, internaluser.ChangePasswordCommand{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

func (h *Controller) ListUsers(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	var req v1.ListUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	result, err := h.service.ListUsers(c.Request.Context(), actor, internaluser.ListQuery{
		Page:     req.Page,
		PageSize: req.PageSize,
		Role:     req.Role,
		Status:   req.Status,
		Keyword:  req.Keyword,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListUsersResponse(result))
}

func (h *Controller) GetUser(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := userIDParam(c)
	if !ok {
		return
	}
	currentUser, err := h.service.GetUser(c.Request.Context(), actor, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toUserResponse(currentUser))
}

func (h *Controller) UpdateUser(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := userIDParam(c)
	if !ok {
		return
	}
	var req v1.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	currentUser, err := h.service.UpdateUser(c.Request.Context(), actor, id, internaluser.AdminUpdateCommand{
		Username: req.Username,
		Email:    req.Email,
		Avatar:   req.Avatar,
		Bio:      req.Bio,
		Status:   req.Status,
		Role:     req.Role,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toUserResponse(currentUser))
}

func (h *Controller) DeleteUser(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := userIDParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteUser(c.Request.Context(), actor, id); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

func (h *Controller) ResetPassword(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := userIDParam(c)
	if !ok {
		return
	}
	var req v1.AdminResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), actor, id, req.Password); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

func userIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.Error(c, apperrors.InvalidRequest)
		return 0, false
	}
	return id, true
}

func writeServiceError(c *gin.Context, err error) {
	appErr := apperrors.From(err)
	if appErr == apperrors.InternalError || appErr.Code == apperrors.CodeInternalError {
		klog.ErrorS(err, "user request failed")
	}
	common.Error(c, appErr)
}
