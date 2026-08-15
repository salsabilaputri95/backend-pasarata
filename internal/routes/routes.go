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
	authenticatedGroup.PUT("/entries/:id", collectorHandler.UpdateEntry)
	authenticatedGroup.PATCH("/entries/:id/deactivate", collectorHandler.DeactivateEntry)
	authenticatedGroup.GET("/markets", adminHandler.GetMarkets)
	authenticatedGroup.GET("/categories", adminHandler.GetCategories)
	authenticatedGroup.GET("/commodities", adminHandler.GetCommodities)
	authenticatedGroup.GET("/units", adminHandler.GetUnits)
	authenticatedGroup.GET("/price-reference", collectorHandler.GetPriceReference)

	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.RequireAuth(cfg), middleware.RequireRole("admin"))
	adminGroup.GET("/dashboard", adminHandler.Dashboard)
	adminGroup.GET("/collectors", adminHandler.GetCollectors)
	adminGroup.POST("/collectors", adminHandler.CreateCollector)
	adminGroup.PUT("/collectors/:id", adminHandler.UpdateCollector)
	adminGroup.PATCH("/collectors/:id/status", adminHandler.SetCollectorStatus)
	adminGroup.POST("/collectors/:id/reset-password", adminHandler.ResetCollectorPassword)
	adminGroup.GET("/markets", adminHandler.GetMarkets)
	adminGroup.POST("/markets", adminHandler.CreateMarket)
	adminGroup.PUT("/markets/:id", adminHandler.UpdateMarket)
	adminGroup.PATCH("/markets/:id/status", adminHandler.SetMarketStatus)
	adminGroup.GET("/assignments", adminHandler.GetAssignments)
	adminGroup.POST("/assignments", adminHandler.CreateAssignment)
	adminGroup.DELETE("/assignments/:id", adminHandler.DeleteAssignment)
	adminGroup.GET("/categories", adminHandler.GetCategories)
	adminGroup.POST("/categories", adminHandler.CreateCategory)
	adminGroup.PUT("/categories/:id", adminHandler.UpdateCategory)
	adminGroup.PATCH("/categories/:id/status", adminHandler.SetCategoryStatus)
	adminGroup.GET("/commodities", adminHandler.GetCommodities)
	adminGroup.POST("/commodities", adminHandler.CreateCommodity)
	adminGroup.PUT("/commodities/:id", adminHandler.UpdateCommodity)
	adminGroup.PATCH("/commodities/:id/status", adminHandler.SetCommodityStatus)
	adminGroup.GET("/units", adminHandler.GetUnits)
	adminGroup.POST("/units", adminHandler.CreateUnit)
	adminGroup.PUT("/units/:id", adminHandler.UpdateUnit)
	adminGroup.PATCH("/units/:id/status", adminHandler.SetUnitStatus)
	adminGroup.GET("/entries", adminHandler.GetEntriesFiltered)
	adminGroup.PUT("/entries/:id", adminHandler.AdminUpdateEntry)
	adminGroup.DELETE("/entries/:id", adminHandler.AdminDeleteEntry)
	adminGroup.GET("/audit-logs", adminHandler.GetAuditLogs)
	adminGroup.GET("/comparison", adminHandler.GetComparison)
	adminGroup.GET("/summary", adminHandler.GetSummary)
	adminGroup.GET("/export", adminHandler.ExportCSV)
	adminGroup.GET("/export-report", adminHandler.ExportReport)
	adminGroup.POST("/import/preview", adminHandler.PreviewImportEntries)
	adminGroup.POST("/import/commit", adminHandler.ImportEntries)
}
