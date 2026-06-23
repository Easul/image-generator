package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/auth"
	"image-generator/internal/config"
	"image-generator/internal/middleware"
)

type UserHandler struct {
	DB     *gorm.DB
	Config config.Config
	Auth   *auth.Service
}

func NewUserHandler(db *gorm.DB, cfg config.Config) *UserHandler {
	return &UserHandler{DB: db, Config: cfg, Auth: auth.NewService(db)}
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *UserHandler) Profile(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(user)})
}

func (h *UserHandler) AppConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"site_name":      settingValue(h.DB, "site_name", h.Config.Defaults.SiteName),
		"site_icon":      settingValue(h.DB, "site_icon", h.Config.Defaults.SiteIcon),
		"allow_register": boolSetting(h.DB, "allow_register", h.Config.Defaults.AllowRegister),
		"models":         stringListSetting(h.DB, "available_models", h.Config.Defaults.AvailableModels),
		"sizes":          stringListSetting(h.DB, "available_sizes", h.Config.Defaults.AvailableSizes),
	})
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	if err := h.Auth.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码不正确"})
		case auth.ErrWeakCredentials:
			c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少 6 位"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func stringListSetting(db *gorm.DB, key string, fallback []string) []string {
	value := settingValue(db, key, "")
	if value == "" {
		return fallback
	}
	var list []string
	if err := json.Unmarshal([]byte(value), &list); err != nil || len(list) == 0 {
		return fallback
	}
	return list
}
