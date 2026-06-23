package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/middleware"
	"image-generator/internal/model"
	"image-generator/internal/service"
)

type ImageHandler struct {
	DB      *gorm.DB
	Service *service.ImageService
}

type generateRequest struct {
	Prompt     string `json:"prompt"`
	Model      string `json:"model"`
	Ratio      string `json:"ratio"`
	Resolution string `json:"resolution"`
	Size       string `json:"size"`
}

func NewImageHandler(db *gorm.DB, imageService *service.ImageService) *ImageHandler {
	return &ImageHandler{DB: db, Service: imageService}
}

func (h *ImageHandler) Generate(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	image, err := h.Service.Generate(c.Request.Context(), user.ID, service.GenerateRequest{
		Prompt:     req.Prompt,
		Model:      req.Model,
		Ratio:      req.Ratio,
		Resolution: firstNonEmpty(req.Resolution, req.Size),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "image": imagePayload(image)})
		return
	}
	c.JSON(http.StatusOK, imagePayload(image))
}

func (h *ImageHandler) Edit(c *gin.Context) {
	h.handleUploadTask(c, "edit")
}

func (h *ImageHandler) RemoveBackground(c *gin.Context) {
	h.handleUploadTask(c, "remove-bg")
}

func (h *ImageHandler) Task(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务 ID 不正确"})
		return
	}
	var image model.Image
	if err := h.DB.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	if image.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该任务"})
		return
	}
	c.JSON(http.StatusOK, imagePayload(&image))
}

func (h *ImageHandler) handleUploadTask(c *gin.Context, taskType string) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传图片"})
		return
	}
	prompt := strings.TrimSpace(c.PostForm("prompt"))
	if taskType == "remove-bg" && prompt == "" {
		prompt = "remove background"
	}
	image, err := h.Service.Edit(c.Request.Context(), user.ID, service.EditRequest{
		TaskType:   taskType,
		Prompt:     prompt,
		Model:      c.PostForm("model"),
		Ratio:      c.PostForm("ratio"),
		Resolution: firstNonEmpty(c.PostForm("resolution"), c.PostForm("size")),
		File:       file,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "image": imagePayload(image)})
		return
	}
	c.JSON(http.StatusOK, imagePayload(image))
}

func imagePayload(image *model.Image) gin.H {
	if image == nil {
		return gin.H{}
	}
	payload := gin.H{
		"id":           image.ID,
		"user_id":      image.UserID,
		"task_type":    image.TaskType,
		"prompt":       image.Prompt,
		"model":        image.Model,
		"ratio":        image.Ratio,
		"resolution":   image.Resolution,
		"source_image": image.SourceImage,
		"status":       image.Status,
		"created_at":   image.CreatedAt,
	}
	if image.ImageURL != "" || image.LocalPath != "" {
		payload["image"] = "/api/image/proxy?id=" + strconv.FormatUint(uint64(image.ID), 10)
	}
	return payload
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
