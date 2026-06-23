package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var store *sessions.CookieStore
var sessionName = "image_workbench_session"

func InitSession(secret, name string, secure bool) {
	if secret == "" {
		secret = "change-me-to-a-long-random-secret"
	}
	if name != "" {
		sessionName = name
	}
	store = sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
}

func GetSession(c *gin.Context) (*sessions.Session, error) {
	if store == nil {
		InitSession("", "", false)
	}
	return store.Get(c.Request, sessionName)
}

func SetUserSession(c *gin.Context, userID uint) error {
	session, err := GetSession(c)
	if err != nil {
		return err
	}
	session.Values["user_id"] = int(userID)
	return session.Save(c.Request, c.Writer)
}

func ClearUserSession(c *gin.Context) error {
	session, err := GetSession(c)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	delete(session.Values, "user_id")
	return session.Save(c.Request, c.Writer)
}

func SessionUserID(c *gin.Context) (uint, bool) {
	session, err := GetSession(c)
	if err != nil {
		return 0, false
	}
	value, ok := session.Values["user_id"]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return uint(typed), typed > 0
	case int64:
		return uint(typed), typed > 0
	case uint:
		return typed, typed > 0
	case uint64:
		return uint(typed), typed > 0
	case float64:
		return uint(typed), typed > 0
	default:
		return 0, false
	}
}
