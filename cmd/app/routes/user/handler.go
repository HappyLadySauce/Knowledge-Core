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
	return &Controller{service: internaluser.NewService(sc.DB, sc.Auth)}
}

// Init registers user routes.
// Init 注册用户路由。
func Init(ctx context.Context, sc *svc.ServiceContext) {
	_ = ctx
	RegisterRoutes(router.V1(), internaluser.NewService(sc.DB, sc.Auth), sc)
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

// Me returns the current authenticated user.
// Me 返回当前认证用户。
// @Summary Get current user
// @Description Return the authenticated user's public profile.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} common.SwaggerResponse{data=v1.UserResponse}
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/users/me [get]
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

// UpdateMe updates the current authenticated user's profile.
// UpdateMe 更新当前认证用户资料。
// @Summary Update current user
// @Description Update username, email, avatar, or bio for the authenticated user.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body v1.UpdateMeRequest true "Profile update request"
// @Success 200 {object} common.SwaggerResponse{data=v1.UserResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/users/me [put]
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

// ChangePassword changes the current authenticated user's password.
// ChangePassword 修改当前认证用户密码。
// @Summary Change current user password
// @Description Verify the old password, set a new password, and revoke existing refresh tokens.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body v1.ChangePasswordRequest true "Password change request"
// @Success 200 {object} common.SwaggerResponse
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/users/me/password [put]
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

// ListUsers returns paginated users for admin management.
// ListUsers 返回 admin 管理用的分页用户列表。
// @Summary List users
// @Description List users with optional role, status, and keyword filters. Admin only.
// @Tags Admin Users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param role query string false "Role filter" Enums(admin,user)
// @Param status query string false "Status filter" Enums(active,disabled)
// @Param keyword query string false "Keyword filter for username or email"
// @Success 200 {object} common.SwaggerResponse{data=v1.ListUsersResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/users [get]
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

// GetUser returns one user by id for admin management.
// GetUser 返回 admin 管理用的单个用户。
// @Summary Get user
// @Description Get one user by id. Admin only.
// @Tags Admin Users
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} common.SwaggerResponse{data=v1.UserResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/users/{id} [get]
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

// UpdateUser updates one user for admin management.
// UpdateUser 更新 admin 管理的单个用户。
// @Summary Update user
// @Description Update username, email, avatar, bio, status, or role. Admin only.
// @Tags Admin Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body v1.AdminUpdateUserRequest true "Admin user update request"
// @Success 200 {object} common.SwaggerResponse{data=v1.UserResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/users/{id} [patch]
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

// DeleteUser disables one user for admin management.
// DeleteUser 禁用 admin 管理的单个用户。
// @Summary Delete user
// @Description Soft-delete a user by setting status to disabled and revoking refresh tokens. Admin only.
// @Tags Admin Users
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} common.SwaggerResponse
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/users/{id} [delete]
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

// ResetPassword resets one user's password for admin management.
// ResetPassword 重置 admin 管理的单个用户密码。
// @Summary Reset user password
// @Description Reset another user's password and revoke that user's refresh tokens. Admin only.
// @Tags Admin Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body v1.AdminResetPasswordRequest true "Admin password reset request"
// @Success 200 {object} common.SwaggerResponse
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/users/{id}/password [put]
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
