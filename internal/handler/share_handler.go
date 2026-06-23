package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"image-generator/internal/middleware"
	"image-generator/internal/service"
)

type ShareHandler struct {
	Service *service.ShareService
}

type createShareRequest struct {
	ImageID       uint `json:"image_id"`
	ExpiresInDays int  `json:"expires_in_days"`
}

func NewShareHandler(shareService *service.ShareService) *ShareHandler {
	return &ShareHandler{Service: shareService}
}

func (h *ShareHandler) Create(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var req createShareRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ImageID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片 ID 不正确"})
		return
	}
	share, err := h.Service.CreateShare(user.ID, req.ImageID, req.ExpiresInDays)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrShareForbidden) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      share.ShareToken,
		"url":        "/share.html?token=" + share.ShareToken,
		"expires_at": share.ExpiresAt,
	})
}

func (h *ShareHandler) Get(c *gin.Context) {
	share, err := h.Service.GetShare(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在或已过期"})
		return
	}
	payload := imagePayload(&share.Image)
	payload["image"] = "/api/image/proxy?id=" + strconv.FormatUint(uint64(share.ImageID), 10) + "&token=" + share.ShareToken
	c.JSON(http.StatusOK, gin.H{
		"token":      share.ShareToken,
		"created_at": share.CreatedAt,
		"expires_at": share.ExpiresAt,
		"image":      payload,
	})
}
