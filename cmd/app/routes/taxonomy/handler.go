package taxonomy

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
	internaltaxonomy "github.com/HappyLadySauce/Knowledge-Core/internal/taxonomy"
)

type Controller struct {
	service internaltaxonomy.TaxonomyService
}

func Init(ctx context.Context, sc *svc.ServiceContext) {
	_ = ctx
	RegisterRoutes(router.V1(), internaltaxonomy.NewService(sc.DB), sc)
}

func RegisterRoutes(group *gin.RouterGroup, service internaltaxonomy.TaxonomyService, sc *svc.ServiceContext) {
	controller := &Controller{service: service}
	group.GET("/categories", controller.ListCategories)
	group.GET("/tags", controller.ListTags)

	adminGroup := group.Group("/admin", middleware.AuthMiddleware(sc), middleware.RequireAdmin())
	adminGroup.GET("/categories", controller.ListAdminCategories)
	adminGroup.POST("/categories", controller.CreateCategory)
	adminGroup.PATCH("/categories/:id", controller.UpdateCategory)
	adminGroup.DELETE("/categories/:id", controller.DeleteCategory)
	adminGroup.GET("/tags", controller.ListAdminTags)
	adminGroup.POST("/tags", controller.CreateTag)
	adminGroup.PATCH("/tags/:id", controller.UpdateTag)
	adminGroup.DELETE("/tags/:id", controller.DeleteTag)
}

// ListCategories returns category tree.
// ListCategories 返回分类树。
// @Summary List categories
// @Description List categories as a tree.
// @Tags Categories
// @Produce json
// @Success 200 {object} common.SwaggerResponse{data=v1.ListCategoriesResponse}
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/categories [get]
func (h *Controller) ListCategories(c *gin.Context) {
	items, err := h.service.ListPublicCategories(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListCategoriesResponse(items))
}

// ListAdminCategories returns category tree for admin.
// ListAdminCategories 返回管理员分类树。
// @Summary List categories
// @Description List categories as a tree. Admin only.
// @Tags Admin Categories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} common.SwaggerResponse{data=v1.ListCategoriesResponse}
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/categories [get]
func (h *Controller) ListAdminCategories(c *gin.Context) {
	items, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListCategoriesResponse(items))
}

// CreateCategory creates a category.
// CreateCategory 创建分类。
// @Summary Create category
// @Description Create a category. Admin only.
// @Tags Admin Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body v1.CreateCategoryRequest true "Category create request"
// @Success 201 {object} common.SwaggerResponse{data=v1.CategoryResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/categories [post]
func (h *Controller) CreateCategory(c *gin.Context) {
	var req v1.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	item, err := h.service.CreateCategory(c.Request.Context(), internaltaxonomy.CategoryCommand{
		Name:     req.Name,
		Slug:     req.Slug,
		ParentID: req.ParentID,
		Sort:     req.Sort,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.Created(c, toCategoryResponse(item))
}

// UpdateCategory updates a category.
// UpdateCategory 更新分类。
// @Summary Update category
// @Description Update a category. Admin only.
// @Tags Admin Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Category ID"
// @Param request body v1.UpdateCategoryRequest true "Category update request"
// @Success 200 {object} common.SwaggerResponse{data=v1.CategoryResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/categories/{id} [patch]
func (h *Controller) UpdateCategory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req v1.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	item, err := h.service.UpdateCategory(c.Request.Context(), id, internaltaxonomy.CategoryUpdateCommand{
		Name:     req.Name,
		Slug:     req.Slug,
		ParentID: req.ParentID,
		Sort:     req.Sort,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toCategoryResponse(item))
}

// DeleteCategory deletes a category.
// DeleteCategory 删除分类。
// @Summary Delete category
// @Description Delete an empty leaf category. Admin only.
// @Tags Admin Categories
// @Produce json
// @Security BearerAuth
// @Param id path int true "Category ID"
// @Success 200 {object} common.SwaggerResponse
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/categories/{id} [delete]
func (h *Controller) DeleteCategory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteCategory(c.Request.Context(), id); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

// ListTags returns tags.
// ListTags 返回标签列表。
// @Summary List tags
// @Description List tags ordered by usage.
// @Tags Tags
// @Produce json
// @Success 200 {object} common.SwaggerResponse{data=v1.ListTagsResponse}
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/tags [get]
func (h *Controller) ListTags(c *gin.Context) {
	items, err := h.service.ListPublicTags(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListTagsResponse(items))
}

// ListAdminTags returns tags for admin.
// ListAdminTags 返回管理员标签列表。
// @Summary List tags
// @Description List tags ordered by usage. Admin only.
// @Tags Admin Tags
// @Produce json
// @Security BearerAuth
// @Success 200 {object} common.SwaggerResponse{data=v1.ListTagsResponse}
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/tags [get]
func (h *Controller) ListAdminTags(c *gin.Context) {
	items, err := h.service.ListTags(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toListTagsResponse(items))
}

// CreateTag creates a tag.
// CreateTag 创建标签。
// @Summary Create tag
// @Description Create a tag. Admin only.
// @Tags Admin Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body v1.CreateTagRequest true "Tag create request"
// @Success 201 {object} common.SwaggerResponse{data=v1.TagResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/tags [post]
func (h *Controller) CreateTag(c *gin.Context) {
	var req v1.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	item, err := h.service.CreateTag(c.Request.Context(), internaltaxonomy.TagCommand{
		Name: req.Name,
		Slug: req.Slug,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.Created(c, toTagResponse(item))
}

// UpdateTag updates a tag.
// UpdateTag 更新标签。
// @Summary Update tag
// @Description Update a tag. Admin only.
// @Tags Admin Tags
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tag ID"
// @Param request body v1.UpdateTagRequest true "Tag update request"
// @Success 200 {object} common.SwaggerResponse{data=v1.TagResponse}
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/tags/{id} [patch]
func (h *Controller) UpdateTag(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req v1.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, apperrors.InvalidRequest)
		return
	}
	item, err := h.service.UpdateTag(c.Request.Context(), id, internaltaxonomy.TagUpdateCommand{
		Name: req.Name,
		Slug: req.Slug,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK(c, toTagResponse(item))
}

// DeleteTag deletes a tag.
// DeleteTag 删除标签。
// @Summary Delete tag
// @Description Delete an unused tag. Admin only.
// @Tags Admin Tags
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tag ID"
// @Success 200 {object} common.SwaggerResponse
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Failure 404 {object} common.SwaggerErrorResponse
// @Failure 409 {object} common.SwaggerErrorResponse
// @Failure 500 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/tags/{id} [delete]
func (h *Controller) DeleteTag(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteTag(c.Request.Context(), id); err != nil {
		writeServiceError(c, err)
		return
	}
	common.OK[any](c, nil)
}

func idParam(c *gin.Context) (int64, bool) {
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
		klog.ErrorS(err, "taxonomy request failed")
	}
	common.Error(c, appErr)
}
