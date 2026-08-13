package main

import (
	"log"
	"net/http"
	"time"

	"backend-pasarata/internal/config"
	"backend-pasarata/internal/db"
	"backend-pasarata/internal/models"
	"backend-pasarata/internal/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	gormDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	if err := seedAdmin(gormDB); err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL, "http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	routes.SetupRoutes(api, cfg, gormDB)

	if err := r.Run("0.0.0.0:" + cfg.Port); err != nil {
		log.Fatalf("server start: %v", err)
	}
}

func seedAdmin(gormDB *gorm.DB) error {
	var existing models.User
	if err := gormDB.Where("username = ?", "admin").First(&existing).Error; err == nil {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		return err
	}

	admin := models.User{
		Username:     "admin",
		PasswordHash: string(hash),
		FullName:     "Administrator",
		Role:         models.RoleAdmin,
		Status:       models.UserStatusActive,
	}

	return gormDB.Create(&admin).Error
}
