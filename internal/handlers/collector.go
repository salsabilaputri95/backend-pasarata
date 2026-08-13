package handlers

import (
	"net/http"
	"time"

	"backend-pasarata/internal/models"
	"backend-pasarata/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CollectorHandler struct {
	DB *gorm.DB
}

type EntryCreateRequest struct {
	Year           int     `json:"year" binding:"required"`
	MarketID       int     `json:"market_id" binding:"required"`
	CategoryID     int     `json:"category_id" binding:"required"`
	CommodityID    int     `json:"commodity_id" binding:"required"`
	BrandType      string  `json:"brand_type"`
	LocalUnitID    int     `json:"local_unit_id" binding:"required"`
	LocalQuantity  float64 `json:"local_quantity" binding:"required"`
	LocalWeightKg  float64 `json:"local_weight_kg" binding:"required"`
	StandardUnitID int     `json:"standard_unit_id" binding:"required"`
	MarketPrice    float64 `json:"market_price" binding:"required"`
	MinimumPrice   float64 `json:"minimum_price" binding:"required"`
	MaximumPrice   float64 `json:"maximum_price" binding:"required"`
	PreviousPrice  float64 `json:"previous_price"`
	Notes          string  `json:"notes"`
}

func (h *CollectorHandler) Dashboard(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)

	var entries []models.DataEntry
	var count int64
	if err := h.DB.Where("collector_id = ?", collectorID).Find(&entries).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load dashboard"})
		return
	}

	warningCount := 0
	for _, entry := range entries {
		if entry.WarningStatus != "normal" {
			warningCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"collector_id":    collectorID,
		"total_entries":   count,
		"warning_entries": warningCount,
		"entries":         entries,
	})
}

func (h *CollectorHandler) CreateEntry(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)
	var req EntryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	if err := services.ValidateEntry(services.EntryInput{
		Year:           req.Year,
		MarketID:       req.MarketID,
		CategoryID:     req.CategoryID,
		CommodityID:    req.CommodityID,
		BrandType:      req.BrandType,
		LocalUnitID:    req.LocalUnitID,
		LocalQuantity:  req.LocalQuantity,
		LocalWeightKg:  req.LocalWeightKg,
		StandardUnitID: req.StandardUnitID,
		MarketPrice:    req.MarketPrice,
		MinimumPrice:   req.MinimumPrice,
		MaximumPrice:   req.MaximumPrice,
		PreviousPrice:  req.PreviousPrice,
		Notes:          req.Notes,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	standardWeight := 1.0
	converted := services.CalculateConvertedPrice(req.MarketPrice, req.LocalWeightKg, standardWeight)
	warning := services.WarningStatus(req.MarketPrice, req.MinimumPrice, req.MaximumPrice)

	entry := models.DataEntry{
		Year:             req.Year,
		MarketID:         req.MarketID,
		CollectorID:      collectorID,
		CategoryID:       req.CategoryID,
		CommodityID:      req.CommodityID,
		BrandType:        req.BrandType,
		LocalUnitID:      req.LocalUnitID,
		LocalQuantity:    req.LocalQuantity,
		LocalWeightKg:    req.LocalWeightKg,
		StandardUnitID:   req.StandardUnitID,
		StandardQuantity: req.LocalQuantity,
		MarketPrice:      req.MarketPrice,
		MinimumPrice:     req.MinimumPrice,
		MaximumPrice:     req.MaximumPrice,
		PreviousPrice:    req.PreviousPrice,
		ConvertedPrice:   converted,
		WarningStatus:    warning,
		Notes:            req.Notes,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := h.DB.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save data"})
		return
	}

	if err := h.DB.Create(&models.AuditLog{
		EntryID:   entry.ID,
		UserID:    collectorID,
		Action:    "create",
		Before:    "",
		After:     "entry created",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save audit log"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "entry created", "data": entry})
}

func (h *CollectorHandler) MyEntries(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)
	var entries []models.DataEntry
	if err := h.DB.Where("collector_id = ?", collectorID).Order("created_at DESC").Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load entries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}
