package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/middleware"
	"image-generator/internal/model"
)

type batchDeleteRequest struct {
	IDs []uint `json:"ids"`
}

type GalleryHandler struct {
	DB *gorm.DB
}

func NewGalleryHandler(db *gorm.DB) *GalleryHandler {
	return &GalleryHandler{DB: db}
}

func (h *GalleryHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	query := h.DB.Order("created_at desc")
	if !(user.IsAdmin && c.Query("all") == "1") {
		query = query.Where("user_id = ?", user.ID)
	}
	var images []model.Image
	if err := query.Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取画廊失败"})
		return
	}
	items := make([]gin.H, 0, len(images))
	for index := range images {
		payload := imagePayload(&images[index])
		if _, ok := payload["image"]; !ok {
			continue
		}
		items = append(items, payload)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *GalleryHandler) Get(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片 ID 不正确"})
		return
	}
	var image model.Image
	if err := h.DB.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	if image.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该图片"})
		return
	}
	c.JSON(http.StatusOK, imagePayload(&image))
}

func (h *GalleryHandler) Delete(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片 ID 不正确"})
		return
	}
	var image model.Image
	if err := h.DB.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	if image.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该图片"})
		return
	}
	if image.LocalPath != "" {
		_ = os.Remove(image.LocalPath)
	}
	if image.SourceImage != "" {
		_ = os.Remove(image.SourceImage)
	}
	_ = h.DB.Where("image_id = ?", image.ID).Delete(&model.Share{}).Error
	if err := h.DB.Delete(&image).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除图片失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *GalleryHandler) BatchDelete(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var req batchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要删除的图片"})
		return
	}
	var images []model.Image
	query := h.DB.Where("id IN ?", req.IDs)
	if !user.IsAdmin {
		query = query.Where("user_id = ?", user.ID)
	}
	if err := query.Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图片失败"})
		return
	}
	for _, image := range images {
		if image.LocalPath != "" {
			_ = os.Remove(image.LocalPath)
		}
		if image.SourceImage != "" {
			_ = os.Remove(image.SourceImage)
		}
	}
	ids := make([]uint, 0, len(images))
	for _, image := range images {
		ids = append(ids, image.ID)
	}
	if len(ids) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到可删除的图片"})
		return
	}
	_ = h.DB.Where("image_id IN ?", ids).Delete(&model.Share{}).Error
	if err := h.DB.Where("id IN ?", ids).Delete(&model.Image{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除图片失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": len(images)})
}
