package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"image-generator/internal/middleware"
	"image-generator/internal/model"
	"image-generator/internal/proxy"
)

type ProxyHandler struct {
	ImageProxy *proxy.ImageProxy
}

func NewProxyHandler(imageProxy *proxy.ImageProxy) *ProxyHandler {
	return &ProxyHandler{ImageProxy: imageProxy}
}

func (h *ProxyHandler) Proxy(c *gin.Context) {
	if !h.canAccess(c) {
		return
	}
	h.ImageProxy.Serve(c)
}

func (h *ProxyHandler) canAccess(c *gin.Context) bool {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少图片 ID"})
		return false
	}

	var image model.Image
	if err := h.ImageProxy.DB.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return false
	}

	if userID, ok := middleware.SessionUserID(c); ok {
		var user model.User
		if err := h.ImageProxy.DB.First(&user, userID).Error; err == nil && (user.IsAdmin || image.UserID == user.ID) {
			return true
		}
	}

	token := c.Query("token")
	if token != "" {
		var share model.Share
		err := h.ImageProxy.DB.Where("image_id = ? AND share_token = ?", image.ID, token).First(&share).Error
		if err == nil && (share.ExpiresAt == nil || time.Now().Before(*share.ExpiresAt)) {
			return true
		}
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "无权访问该图片"})
	return false
}
