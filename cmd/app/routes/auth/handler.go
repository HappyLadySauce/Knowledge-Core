package auth

import (
	"context"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/router"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/svc"
	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/common"
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	"github.com/HappyLadySauce/Knowledge-Core/internal/auth"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type Controller struct {
	service auth.AuthService
}

func NewController(sc *svc.ServiceContext) *Controller {
	return &Controller{service: auth.NewService(sc.DB, sc.Config.JWT)}
}

// Init registers auth routes.
// Init 注册认证路由。
func Init(ctx context.Context, sc *svc.ServiceContext) {
	_ = ctx
	RegisterRoutes(router.V1(), auth.NewService(sc.DB, sc.Config.JWT), sc)
}

// RegisterRoutes attaches auth and admin routes to the API group.
// RegisterRoutes 将认证与 admin 路由挂载到 API 分组。
func RegisterRoutes(group *gin.RouterGroup, service auth.AuthService, sc *svc.ServiceContext) {
	_ = sc
	controller := NewController(sc)
	authGroup := group.Group("/auth")
	authGroup.POST("/register", controller.Register)
	authGroup.POST("/login", controller.Login)
	authGroup.POST("/refresh", controller.Refresh)
	authGroup.POST("/logout", controller.Logout)
}

func (h *Controller) Register(c *gin.Context) {
	var req v1.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	response, err := h.service.Register(c.Request.Context(), auth.RegisterCommand{
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.Created(c, toTokenResponse(response))
}

func (h *Controller) Login(c *gin.Context) {
	var req v1.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	response, err := h.service.Login(c.Request.Context(), auth.LoginCommand{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toTokenResponse(response))
}

func (h *Controller) Refresh(c *gin.Context) {
	var req v1.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	response, err := h.service.Refresh(c.Request.Context(), auth.RefreshCommand{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toTokenResponse(response))
}

func (h *Controller) Logout(c *gin.Context) {
	var req v1.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	if err := h.service.Logout(c.Request.Context(), auth.LogoutCommand{
		RefreshToken: req.RefreshToken,
	}); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

func writeServiceError(c *gin.Context, err error) {
	appErr := apperrors.From(err)
	if appErr == apperrors.InternalError || appErr.Code == apperrors.CodeInternalError {
		klog.ErrorS(err, "auth request failed")
	}
	common.Error(c, apperrors.From(err))
}
