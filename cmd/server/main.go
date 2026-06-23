package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"image-generator/internal/config"
	"image-generator/internal/database"
	"image-generator/internal/handler"
	"image-generator/internal/middleware"
	proxyPkg "image-generator/internal/proxy"
	"image-generator/internal/service"
)

//go:embed web/assets/*
var assetsFS embed.FS

//go:embed web/*.html
var htmlFS embed.FS

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := os.MkdirAll(cfg.Storage.OriginalDir, 0o755); err != nil {
		log.Fatalf("create original upload directory: %v", err)
	}
	if err := os.MkdirAll(cfg.Storage.GeneratedDir, 0o755); err != nil {
		log.Fatalf("create generated upload directory: %v", err)
	}

	db, err := database.Init(cfg)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}

	middleware.InitSession(cfg.Server.SessionSecret, cfg.Server.SessionName, cfg.Server.SecureCookies)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.MaxMultipartMemory = cfg.Storage.MaxUploadMB << 20
	if err := router.SetTrustedProxies([]string{"127.0.0.1", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}); err != nil {
		log.Fatalf("set trusted proxies: %v", err)
	}

	assetsSub, err := fs.Sub(assetsFS, "web/assets")
	if err != nil {
		log.Fatalf("load embedded assets: %v", err)
	}
	router.StaticFS("/assets", http.FS(assetsSub))

	registerHTML(router, "/", "index.html")
	registerHTML(router, "/index.html", "index.html")
	registerHTML(router, "/login.html", "login.html")
	registerHTML(router, "/register.html", "register.html")
	registerHTML(router, "/gallery.html", "gallery.html")
	registerHTML(router, "/admin.html", "admin.html")
	registerHTML(router, "/password.html", "password.html")
	registerHTML(router, "/share.html", "share.html")

	imageService := service.NewImageService(db, cfg)
	shareService := service.NewShareService(db)
	imageProxy := proxyPkg.NewImageProxy(db, cfg.Storage.GeneratedDir)

	authHandler := handler.NewAuthHandler(db)
	userHandler := handler.NewUserHandler(db, cfg)
	imageHandler := handler.NewImageHandler(db, imageService)
	galleryHandler := handler.NewGalleryHandler(db)
	adminHandler := handler.NewAdminHandler(db, cfg)
	proxyHandler := handler.NewProxyHandler(imageProxy)
	shareHandler := handler.NewShareHandler(shareService)
	apiKeyHandler := handler.NewApiKeyHandler(db)

	api := router.Group("/api")
	api.GET("/config", userHandler.AppConfig)
	api.POST("/register", authHandler.Register)
	api.POST("/login", authHandler.Login)
	api.POST("/logout", authHandler.Logout)
	api.GET("/image/proxy", proxyHandler.Proxy)
	api.GET("/share/:token", shareHandler.Get)

	protected := api.Group("")
	protected.Use(middleware.AuthRequired(db))
	protected.GET("/profile", userHandler.Profile)
	protected.POST("/profile/password", userHandler.ChangePassword)
	protected.POST("/images/generate", imageHandler.Generate)
	protected.POST("/images/edit", imageHandler.Edit)
	protected.POST("/images/remove-bg", imageHandler.RemoveBackground)
	protected.GET("/task/:id", imageHandler.Task)
	protected.GET("/gallery", galleryHandler.List)
	protected.GET("/gallery/:id", galleryHandler.Get)
	protected.DELETE("/gallery/:id", galleryHandler.Delete)
	protected.POST("/gallery/batch-delete", galleryHandler.BatchDelete)
	protected.POST("/share", shareHandler.Create)

	admin := api.Group("/admin")
	admin.Use(middleware.AuthRequired(db), middleware.AdminRequired(db))
	admin.GET("/settings", adminHandler.GetSettings)
	admin.POST("/settings", adminHandler.SaveSettings)
	admin.GET("/users", adminHandler.ListUsers)
	admin.POST("/user/admin", adminHandler.SetUserAdmin)
	admin.POST("/user/unadmin", adminHandler.UnsetUserAdmin)
	admin.POST("/user/ban", adminHandler.BanUser)
	admin.POST("/user/unban", adminHandler.UnbanUser)
	admin.POST("/user/reset-password", adminHandler.ResetUserPassword)
	admin.GET("/models", adminHandler.GetModels)
	admin.POST("/models", adminHandler.SaveModels)
	admin.POST("/test-api", adminHandler.TestAPI)
	admin.GET("/api-keys", apiKeyHandler.List)
	admin.POST("/api-keys", apiKeyHandler.Create)
	admin.DELETE("/api-keys", apiKeyHandler.Delete)

	// API key authenticated endpoints
	apiAuth := api.Group("")
	apiAuth.Use(middleware.ApiKeyAuth(db))
	apiAuth.POST("/v1/images/generate", imageHandler.Generate)
	apiAuth.POST("/v1/images/edit", imageHandler.Edit)
	apiAuth.POST("/v1/images/remove-bg", imageHandler.RemoveBackground)

	log.Printf("AI image workbench listening on %s", cfg.Server.Address)
	if err := router.Run(cfg.Server.Address); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func registerHTML(router *gin.Engine, routePath, fileName string) {
	router.GET(routePath, func(c *gin.Context) {
		data, err := htmlFS.ReadFile("web/" + fileName)
		if err != nil {
			c.String(http.StatusNotFound, "page not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}
