package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"image-generator/internal/auth"
	"image-generator/internal/config"
	"image-generator/internal/middleware"
	"image-generator/internal/model"
)

type AdminHandler struct {
	DB     *gorm.DB
	Config config.Config
}

type adminUserRequest struct {
	UserID uint `json:"user_id"`
}

type modelsRequest struct {
	Models []string `json:"models"`
}

func NewAdminHandler(db *gorm.DB, cfg config.Config) *AdminHandler {
	return &AdminHandler{DB: db, Config: cfg}
}

func (h *AdminHandler) GetSettings(c *gin.Context) {
	apiKey, _ := config.DecryptString(h.Config.Server.SessionSecret, settingValue(h.DB, "api_key", h.Config.OpenAI.APIKey))
	c.JSON(http.StatusOK, gin.H{
		"site_name":      settingValue(h.DB, "site_name", h.Config.Defaults.SiteName),
		"site_icon":      settingValue(h.DB, "site_icon", h.Config.Defaults.SiteIcon),
		"allow_register": boolSetting(h.DB, "allow_register", h.Config.Defaults.AllowRegister),
		"base_url":       settingValue(h.DB, "base_url", h.Config.OpenAI.BaseURL),
		"api_key":        apiKey,
		"models":         stringListSetting(h.DB, "available_models", h.Config.Defaults.AvailableModels),
		"sizes":          stringListSetting(h.DB, "available_sizes", h.Config.Defaults.AvailableSizes),
	})
}

func (h *AdminHandler) SaveSettings(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	allowed := map[string]bool{
		"site_name": true, "site_icon": true, "allow_register": true,
		"base_url": true, "api_key": true, "available_models": true, "available_sizes": true,
	}
	for key, raw := range payload {
		if !allowed[key] {
			continue
		}
		value, err := h.normalizeSetting(key, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := setSettingValue(h.DB, key, value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存设置失败"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) GetModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"models": stringListSetting(h.DB, "available_models", h.Config.Defaults.AvailableModels)})
}

func (h *AdminHandler) SaveModels(c *gin.Context) {
	var req modelsRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Models) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少配置一个模型"})
		return
	}
	models := cleanStringList(req.Models)
	if len(models) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少配置一个模型"})
		return
	}
	data, _ := json.Marshal(models)
	if err := setSettingValue(h.DB, "available_models", string(data)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存模型失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []model.User
	if err := h.DB.Order("id asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户失败"})
		return
	}
	items := make([]gin.H, 0, len(users))
	for index := range users {
		user := userResponse(&users[index])
		user["calls_today"], user["calls_total"] = h.userUsageStats(users[index].ID)
		items = append(items, user)
	}
	c.JSON(http.StatusOK, gin.H{"users": items})
}

func (h *AdminHandler) userUsageStats(userID uint) (int64, int64) {
	var total int64
	h.DB.Model(&model.UsageLog{}).Where("user_id = ?", userID).Count(&total)
	var today int64
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	h.DB.Model(&model.UsageLog{}).Where("user_id = ? AND created_at >= ?", userID, startOfDay).Count(&today)
	return today, total
}

func (h *AdminHandler) SetUserAdmin(c *gin.Context) {
	var req adminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户 ID 不正确"})
		return
	}
	result := h.DB.Model(&model.User{}).Where("id = ?", req.UserID).Update("is_admin", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) UnsetUserAdmin(c *gin.Context) {
	var req adminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户 ID 不正确"})
		return
	}
	var user model.User
	if err := h.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	var adminCount int64
	h.DB.Model(&model.User{}).Where("is_admin = ?", true).Count(&adminCount)
	if user.IsAdmin && adminCount <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统至少需要保留一名管理员"})
		return
	}
	if err := h.DB.Model(&user).Update("is_admin", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	var req adminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户 ID 不正确"})
		return
	}
	operator, ok := currentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	if operator.ID == req.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能封禁当前登录管理员"})
		return
	}
	var target model.User
	if err := h.DB.First(&target, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if target.IsAdmin {
		var adminCount int64
		h.DB.Model(&model.User{}).Where("is_admin = ?", true).Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "系统至少需要保留一名可用管理员"})
			return
		}
	}
	result := h.DB.Model(&model.User{}).Where("id = ?", req.UserID).Update("banned", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "封禁用户失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	var req adminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户 ID 不正确"})
		return
	}
	result := h.DB.Model(&model.User{}).Where("id = ?", req.UserID).Update("banned", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解封用户失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	var req adminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户 ID 不正确"})
		return
	}
	operator, ok := currentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	if operator.ID == req.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请使用修改密码功能处理当前账户"})
		return
	}
	password, err := auth.NewService(h.DB).ResetPassword(req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "password": password})
}

func currentAdmin(c *gin.Context) (*model.User, bool) {
	return middleware.CurrentUser(c)
}

func (h *AdminHandler) TestAPI(c *gin.Context) {
	var payload map[string]string
	_ = c.ShouldBindJSON(&payload)
	baseURL, err := config.NormalizeOpenAIBaseURL(firstNonEmpty(payload["base_url"], settingValue(h.DB, "base_url", h.Config.OpenAI.BaseURL)))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	apiKey := firstNonEmpty(payload["api_key"], settingValue(h.DB, "api_key", h.Config.OpenAI.APIKey))
	apiKey, _ = config.DecryptString(h.Config.Server.SessionSecret, apiKey)
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先配置 Base URL"})
		return
	}

	client := http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "连接失败", "status": resp.StatusCode, "body": string(body)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": resp.StatusCode})
}

func (h *AdminHandler) normalizeSetting(key string, raw any) (string, error) {
	switch key {
	case "allow_register":
		switch value := raw.(type) {
		case bool:
			return strconv.FormatBool(value), nil
		case string:
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return "", err
			}
			return strconv.FormatBool(parsed), nil
		default:
			return "", nil
		}
	case "available_models", "available_sizes":
		list := make([]string, 0)
		switch value := raw.(type) {
		case []any:
			for _, item := range value {
				if text, ok := item.(string); ok {
					list = append(list, text)
				}
			}
		case []string:
			list = append(list, value...)
		case string:
			for _, part := range strings.Split(value, "\n") {
				list = append(list, part)
			}
		}
		list = cleanStringList(list)
		data, _ := json.Marshal(list)
		return string(data), nil
	case "api_key":
		plain := strings.TrimSpace(toString(raw))
		return config.EncryptString(h.Config.Server.SessionSecret, plain)
	case "base_url":
		return config.NormalizeOpenAIBaseURL(toString(raw))
	default:
		return strings.TrimSpace(toString(raw)), nil
	}
}

func setSettingValue(db *gorm.DB, key, value string) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": value}),
	}).Create(&model.Setting{Key: key, Value: value}).Error
}

func cleanStringList(values []string) []string {
	seen := map[string]bool{}
	list := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		list = append(list, value)
	}
	return list
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}
