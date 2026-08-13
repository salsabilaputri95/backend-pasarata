package routes

import (
	"backend-pasarata/internal/config"
	"backend-pasarata/internal/handlers"
	"backend-pasarata/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRoutes(api *gin.RouterGroup, cfg config.Config, db *gorm.DB) {
	authHandler := &handlers.AuthHandler{DB: db, Cfg: cfg}
	collectorHandler := &handlers.CollectorHandler{DB: db}
	adminHandler := &handlers.AdminHandler{DB: db}

	api.POST("/login", authHandler.Login)
	api.GET("/me", middleware.RequireAuth(cfg), authHandler.Me)

	authenticatedGroup := api.Group("/")
	authenticatedGroup.Use(middleware.RequireAuth(cfg))
	authenticatedGroup.GET("/dashboard", collectorHandler.Dashboard)
	authenticatedGroup.GET("/entries/me", collectorHandler.MyEntries)
	authenticatedGroup.POST("/entries", collectorHandler.CreateEntry)
	authenticatedGroup.GET("/markets", adminHandler.GetMarkets)
	authenticatedGroup.GET("/categories", adminHandler.GetCategories)
	authenticatedGroup.GET("/commodities", adminHandler.GetCommodities)
	authenticatedGroup.GET("/units", adminHandler.GetUnits)

	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.RequireAuth(cfg), middleware.RequireRole("admin"))
	adminGroup.GET("/dashboard", adminHandler.Dashboard)
	adminGroup.GET("/collectors", adminHandler.GetCollectors)
	adminGroup.POST("/collectors", adminHandler.CreateCollector)
	adminGroup.GET("/markets", adminHandler.GetMarkets)
	adminGroup.POST("/markets", adminHandler.CreateMarket)
	adminGroup.GET("/assignments", adminHandler.GetAssignments)
	adminGroup.POST("/assignments", adminHandler.CreateAssignment)
	adminGroup.GET("/categories", adminHandler.GetCategories)
	adminGroup.POST("/categories", adminHandler.CreateCategory)
	adminGroup.GET("/commodities", adminHandler.GetCommodities)
	adminGroup.POST("/commodities", adminHandler.CreateCommodity)
	adminGroup.GET("/units", adminHandler.GetUnits)
	adminGroup.POST("/units", adminHandler.CreateUnit)
	adminGroup.GET("/entries", adminHandler.GetEntries)
	adminGroup.GET("/audit-logs", adminHandler.GetAuditLogs)
	adminGroup.GET("/comparison", adminHandler.GetComparison)
	adminGroup.GET("/summary", adminHandler.GetSummary)
}
