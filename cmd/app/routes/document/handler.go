package document

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
	internaldocument "github.com/HappyLadySauce/Knowledge-Core/internal/document"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type Controller struct {
	service internaldocument.DocumentService
}

func Init(ctx context.Context, sc *svc.ServiceContext) error {
	_ = ctx
	service, err := internaldocument.NewService(sc.DB, sc.Config.Library.Path)
	if err != nil {
		return err
	}
	RegisterRoutes(router.V1(), service, sc)
	return nil
}

func RegisterRoutes(group *gin.RouterGroup, service internaldocument.DocumentService, sc *svc.ServiceContext) {
	controller := &Controller{service: service}
	group.GET("/documents", controller.ListPublic)
	group.GET("/documents/:id", controller.GetPublic)

	adminGroup := group.Group("/admin", middleware.AuthMiddleware(sc), middleware.RequireAdmin())
	adminGroup.GET("/documents", controller.ListAdmin)
	adminGroup.POST("/documents", controller.CreateAdmin)
	adminGroup.GET("/documents/:id", controller.GetAdmin)
	adminGroup.PATCH("/documents/:id", controller.UpdateAdmin)
	adminGroup.DELETE("/documents/:id", controller.DeleteAdmin)
}

// ListPublic returns published documents.
// ListPublic 返回已发布文档列表。
// @Summary List published documents
// @Description List published documents with optional keyword, category, and tag filters.
// @Tags Documents
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param q query string false "Keyword"
// @Param category query string false "Category slug or path"
// @Param tag query string false "Tag slug or name"
// @Success 200 {object} common.SwaggerResponse{data=v1.ListDocumentsResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/documents [get]
func (h *Controller) ListPublic(c *gin.Context) {
	var req v1.ListDocumentsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	result, err := h.service.ListPublic(c.Request.Context(), internaldocument.ListQuery{
		Page:     req.Page,
		PageSize: req.PageSize,
		Q:        req.Q,
		Category: req.Category,
		Tag:      req.Tag,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListDocumentsResponse(result))
}

// GetPublic returns one published document.
// GetPublic 返回单篇已发布文档。
// @Summary Get published document
// @Description Get a published document by id.
// @Tags Documents
// @Produce json
// @Param id path int true "Document ID"
// @Success 200 {object} common.SwaggerResponse{data=v1.DocumentResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/documents/{id} [get]
func (h *Controller) GetPublic(c *gin.Context) {
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	detail, err := h.service.GetPublic(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toDocumentDetailResponse(detail))
}

// ListAdmin returns admin document list.
// ListAdmin 返回管理员文档列表。
// @Summary List documents
// @Description List all documents for admin management.
// @Tags Admin Documents
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param q query string false "Keyword"
// @Param category query string false "Category slug or path"
// @Param tag query string false "Tag slug or name"
// @Param status query string false "Status" Enums(draft,published)
// @Success 200 {object} common.SwaggerResponse{data=v1.ListDocumentsResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/documents [get]
func (h *Controller) ListAdmin(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	var req v1.ListDocumentsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	result, err := h.service.ListAdmin(c.Request.Context(), actor, internaldocument.ListQuery{
		Page:     req.Page,
		PageSize: req.PageSize,
		Q:        req.Q,
		Category: req.Category,
		Tag:      req.Tag,
		Status:   req.Status,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListDocumentsResponse(result))
}

// CreateAdmin creates a document.
// CreateAdmin 创建文档。
// @Summary Create document
// @Description Create a Markdown document and index metadata. Admin only.
// @Tags Admin Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body v1.CreateDocumentRequest true "Document create request"
// @Success 201 {object} common.SwaggerResponse{data=v1.DocumentResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/documents [post]
func (h *Controller) CreateAdmin(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	var req v1.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	detail, err := h.service.CreateAdmin(c.Request.Context(), actor, internaldocument.CreateCommand{
		Slug:       req.Slug,
		Title:      req.Title,
		Summary:    req.Summary,
		Content:    req.Content,
		CategoryID: req.CategoryID,
		TagIDs:     req.TagIDs,
		Source:     req.Source,
		Status:     req.Status,
		Confidence: req.Confidence,
		CoverURL:   req.CoverURL,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.Created(c, toDocumentDetailResponse(detail))
}

// GetAdmin returns one document for admin.
// GetAdmin 返回管理员文档详情。
// @Summary Get document
// @Description Get a document by id. Admin only.
// @Tags Admin Documents
// @Produce json
// @Security BearerAuth
// @Param id path int true "Document ID"
// @Success 200 {object} common.SwaggerResponse{data=v1.DocumentResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/documents/{id} [get]
func (h *Controller) GetAdmin(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	detail, err := h.service.GetAdmin(c.Request.Context(), actor, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toDocumentDetailResponse(detail))
}

// UpdateAdmin updates one document.
// UpdateAdmin 更新单篇文档。
// @Summary Update document
// @Description Update document metadata, content, tags, category, or status. Admin only.
// @Tags Admin Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Document ID"
// @Param request body v1.UpdateDocumentRequest true "Document update request"
// @Success 200 {object} common.SwaggerResponse{data=v1.DocumentResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/documents/{id} [patch]
func (h *Controller) UpdateAdmin(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	var req v1.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	detail, err := h.service.UpdateAdmin(c.Request.Context(), actor, id, internaldocument.UpdateCommand{
		Slug:       req.Slug,
		Title:      req.Title,
		Summary:    req.Summary,
		Content:    req.Content,
		CategoryID: req.CategoryID,
		TagIDs:     req.TagIDs,
		Source:     req.Source,
		Status:     req.Status,
		Confidence: req.Confidence,
		CoverURL:   req.CoverURL,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toDocumentDetailResponse(detail))
}

// DeleteAdmin deletes one document.
// DeleteAdmin 删除单篇文档。
// @Summary Delete document
// @Description Delete document metadata and Markdown file. Admin only.
// @Tags Admin Documents
// @Produce json
// @Security BearerAuth
// @Param id path int true "Document ID"
// @Success 200 {object} common.SwaggerResponse
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/documents/{id} [delete]
func (h *Controller) DeleteAdmin(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		common.Error(c, apperrors.InvalidToken)
		return
	}
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteAdmin(c.Request.Context(), actor, id); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

func documentIDParam(c *gin.Context) (int64, bool) {
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
		klog.ErrorS(err, "document request failed")
	}
	common.Error(c, appErr)
}
