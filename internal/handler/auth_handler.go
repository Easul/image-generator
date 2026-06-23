package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/auth"
	"image-generator/internal/middleware"
	"image-generator/internal/model"
)

type AuthHandler struct {
	DB   *gorm.DB
	Auth *auth.Service
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginAttempt struct {
	Count       int
	LockedUntil time.Time
}

var attempts = struct {
	sync.Mutex
	items map[string]loginAttempt
}{items: map[string]loginAttempt{}}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{DB: db, Auth: auth.NewService(db)}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}

	var count int64
	if err := h.DB.Model(&model.User{}).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户失败"})
		return
	}
	if count > 0 && !boolSetting(h.DB, "allow_register", true) {
		c.JSON(http.StatusForbidden, gin.H{"error": "当前系统已关闭注册"})
		return
	}

	user, err := h.Auth.Register(req.Username, req.Password)
	if err != nil {
		status := http.StatusBadRequest
		message := "注册失败"
		if err == auth.ErrWeakCredentials {
			message = "用户名至少 3 位，密码至少 6 位"
		}
		c.JSON(status, gin.H{"error": message})
		return
	}
	if err := middleware.SetUserSession(c, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(user)})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}

	key := c.ClientIP() + "|" + strings.TrimSpace(req.Username)
	if wait, ok := canLogin(key); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录失败次数过多，请稍后再试", "retry_after": wait})
		return
	}

	user, err := h.Auth.Authenticate(req.Username, req.Password)
	if err != nil {
		recordLoginFailure(key)
		message := "用户名或密码错误"
		if err == auth.ErrBanned {
			message = "账号已被封禁"
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": message})
		return
	}
	resetLoginFailures(key)
	if err := middleware.SetUserSession(c, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(user)})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if err := middleware.ClearUserSession(c); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "退出失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func canLogin(key string) (int, bool) {
	attempts.Lock()
	defer attempts.Unlock()
	item := attempts.items[key]
	if time.Now().Before(item.LockedUntil) {
		return int(time.Until(item.LockedUntil).Seconds()), false
	}
	return 0, true
}

func recordLoginFailure(key string) {
	attempts.Lock()
	defer attempts.Unlock()
	item := attempts.items[key]
	item.Count++
	if item.Count >= 5 {
		item.LockedUntil = time.Now().Add(5 * time.Minute)
		item.Count = 0
	}
	attempts.items[key] = item
}

func resetLoginFailures(key string) {
	attempts.Lock()
	defer attempts.Unlock()
	delete(attempts.items, key)
}

func userResponse(user *model.User) gin.H {
	return gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"is_admin":   user.IsAdmin,
		"banned":     user.Banned,
		"created_at": user.CreatedAt,
	}
}

func settingValue(db *gorm.DB, key, fallback string) string {
	var setting model.Setting
	if err := db.First(&setting, "key = ?", key).Error; err != nil {
		return fallback
	}
	return setting.Value
}

func boolSetting(db *gorm.DB, key string, fallback bool) bool {
	value := strings.TrimSpace(settingValue(db, key, strconv.FormatBool(fallback)))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
