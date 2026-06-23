package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/model"
	"image-generator/internal/service"
)

func AuthRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := SessionUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		var user model.User
		if err := db.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
			return
		}
		if user.Banned {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "账号已被封禁"})
			return
		}
		c.Set("user", &user)
		c.Set("user_id", user.ID)
		c.Next()
	}
}

func AdminRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			userID, hasSession := SessionUserID(c)
			if !hasSession || db.First(&user, userID).Error != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
				return
			}
		}
		if !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*model.User, bool) {
	return userFromContext(c)
}

func userFromContext(c *gin.Context) (*model.User, bool) {
	value, ok := c.Get("user")
	if !ok {
		return nil, false
	}
	user, ok := value.(*model.User)
	return user, ok && user != nil
}

// ApiKeyAuth validates API key from Authorization header
func ApiKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			return
		}

		// Support "Bearer <key>" format
		key := strings.TrimPrefix(authHeader, "Bearer ")
		key = strings.TrimSpace(key)

		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
			return
		}

		apiKeyService := service.NewApiKeyService(db)
		userID, err := apiKeyService.ValidateKey(key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			return
		}

		var user model.User
		if err := db.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		if user.Banned {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Account banned"})
			return
		}

		c.Set("user", &user)
		c.Set("user_id", user.ID)
		c.Next()
	}
}
