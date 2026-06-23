package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/middleware"
	"image-generator/internal/service"
)

type ApiKeyHandler struct {
	db            *gorm.DB
	apiKeyService *service.ApiKeyService
}

func NewApiKeyHandler(db *gorm.DB) *ApiKeyHandler {
	return &ApiKeyHandler{
		db:            db,
		apiKeyService: service.NewApiKeyService(db),
	}
}

type CreateApiKeyRequest struct {
	Note string `json:"note"`
}

type ApiKeyResponse struct {
	ID         uint    `json:"id"`
	Key        string  `json:"key"`
	MaskedKey  string  `json:"masked_key"`
	Note       string  `json:"note"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

func (h *ApiKeyHandler) Create(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req CreateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	apiKey, err := h.apiKeyService.GenerateKey(user.ID, req.Note)
	if err != nil {
		if err == service.ErrMaxKeysReached {
			c.JSON(http.StatusBadRequest, gin.H{"error": "最多只能创建5个API密钥"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         apiKey.ID,
		"key":        apiKey.Key,
		"masked_key": maskKey(apiKey.Key),
		"note":       apiKey.Note,
		"created_at": apiKey.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *ApiKeyHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	keys, err := h.apiKeyService.ListKeys(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取失败"})
		return
	}

	response := make([]gin.H, len(keys))
	for i, key := range keys {
		var lastUsed *string
		if key.LastUsedAt != nil {
			formatted := key.LastUsedAt.Format("2006-01-02 15:04:05")
			lastUsed = &formatted
		}

		response[i] = gin.H{
			"id":           key.ID,
			"key":          key.Key,
			"masked_key":   maskKey(key.Key),
			"note":         key.Note,
			"last_used_at": lastUsed,
			"created_at":   key.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	c.JSON(http.StatusOK, gin.H{"keys": response})
}

func (h *ApiKeyHandler) Delete(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	err := h.apiKeyService.DeleteKey(user.ID, req.ID)
	if err != nil {
		if err == service.ErrKeyNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "密钥不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// maskKey masks the middle part of the API key
func maskKey(key string) string {
	if len(key) <= 16 {
		return key
	}
	return key[:12] + "..." + key[len(key)-4:]
}
