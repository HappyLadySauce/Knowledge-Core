package auth

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
	"github.com/HappyLadySauce/Knowledge-Core/internal/auth"
	"github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type AuthController struct {
	service *auth.Service
}

func NewAuthController(sc *svc.ServiceContext) *AuthController {
	return &AuthController{service: auth.NewService(sc.DB, sc.Config.JWT)}
}

// Init registers auth routes.
// Init 注册认证路由。
func Init(ctx context.Context, sc *svc.ServiceContext) {
	_ = ctx
	service := auth.NewService(sc.DB, sc.Config.JWT)
	RegisterRoutes(router.V1(), service, sc)
}

// RegisterRoutes attaches auth and admin routes to the API group.
// RegisterRoutes 将认证与 admin 路由挂载到 API 分组。
func RegisterRoutes(group *gin.RouterGroup, service *auth.Service, sc *svc.ServiceContext) {
	controller := &AuthController{service: service}
	authGroup := group.Group("/auth")
	authGroup.POST("/register", controller.Register)
	authGroup.POST("/login", controller.Login)
	authGroup.POST("/refresh", controller.Refresh)
	authGroup.GET("/me", middleware.AuthMiddleware(sc), controller.Me)

	adminGroup := group.Group("/admin", middleware.AuthMiddleware(sc), middleware.RequireAdmin())
	adminGroup.PATCH("/users/:id", controller.UpdateUser)
}

func (h *AuthController) Register(c *gin.Context) {
	var req v1.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, errors.InvalidRequest)
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

func (h *AuthController) Login(c *gin.Context) {
	var req v1.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, errors.InvalidRequest)
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

func (h *AuthController) Refresh(c *gin.Context) {
	var req v1.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, errors.InvalidRequest)
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

func (h *AuthController) Me(c *gin.Context) {
	user, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, errors.InvalidToken)
		return
	}
	common.OK(c, toUserResponse(user))
}

func (h *AuthController) UpdateUser(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, errors.InvalidToken)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.Error(c, errors.InvalidRequest)
		return
	}
	var req v1.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, errors.InvalidRequest)
		return
	}
	user, err := h.service.UpdateUser(c.Request.Context(), actor, id, auth.UpdateUserCommand{
		Username: req.Username,
		Email:    req.Email,
		Status:   req.Status,
		Role:     req.Role,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toUserResponse(user))
}

func writeServiceError(c *gin.Context, err error) {
	appErr := errors.From(err)
	if appErr == errors.InternalError || appErr.Code == errors.CodeInternalError {
		klog.ErrorS(err, "auth request failed")
	}
	common.Error(c, err)
}
