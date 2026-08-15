package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

type EntryUpdateRequest = EntryCreateRequest

func (h *CollectorHandler) Dashboard(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)

	yearFilter := 0
	if yearQuery := c.Query("year"); yearQuery != "" {
		year, err := strconv.Atoi(yearQuery)
		if err != nil || year < 2020 || year > 2100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return
		}
		yearFilter = year
	}

	var assignments []models.UserMarketAssignment
	if err := h.DB.Preload("Market").Where("user_id = ?", collectorID).Find(&assignments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load assignments"})
		return
	}

	markets := make([]gin.H, 0, len(assignments))
	for _, assignment := range assignments {
		markets = append(markets, gin.H{
			"id":       assignment.MarketID,
			"name":     assignment.Market.Name,
			"district": assignment.Market.District,
			"nks":      assignment.Market.NKS,
		})
	}

	entryQuery := h.DB.Where("collector_id = ?", collectorID)
	if yearFilter > 0 {
		entryQuery = entryQuery.Where("year = ?", yearFilter)
	}

	var entries []models.DataEntry
	if err := entryQuery.Order("created_at DESC").Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load dashboard"})
		return
	}

	var totalActive int64
	var totalInactive int64
	var warningCount int64
	for _, entry := range entries {
		if entry.IsActive {
			totalActive++
			if entry.WarningStatus != "normal" {
				warningCount++
			}
		} else {
			totalInactive++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"collector_id":      collectorID,
		"year":              yearFilter,
		"assigned_markets":  len(assignments),
		"markets":           markets,
		"total_entries":     totalActive,
		"inactive_entries":  totalInactive,
		"warning_entries":   warningCount,
		"editable_entries":  totalActive,
		"entries":           entries,
	})
}

func (h *CollectorHandler) CreateEntry(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)
	var req EntryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	if err := h.ensureMarketAssignment(collectorID, req.MarketID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := validateEntryRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	entry := buildEntryFromRequest(req, collectorID, now)
	entry.IsActive = true
	entry.CreatedAt = now

	if err := h.DB.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save data"})
		return
	}

	if err := h.writeAudit(collectorID, entry.ID, "create", "", entrySnapshot(entry)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save audit log"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "entry created", "data": entry})
}

func (h *CollectorHandler) UpdateEntry(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)
	entryID, err := strconv.Atoi(c.Param("id"))
	if err != nil || entryID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry id"})
		return
	}

	var req EntryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	var entry models.DataEntry
	if err := h.DB.Where("id = ? AND collector_id = ?", entryID, collectorID).First(&entry).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}
	if !entry.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "inactive entry cannot be edited"})
		return
	}

	if err := h.ensureMarketAssignment(collectorID, req.MarketID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := validateEntryRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	before := entrySnapshot(entry)
	now := time.Now()
	updated := buildEntryFromRequest(req, collectorID, now)
	updated.ID = entry.ID
	updated.IsActive = entry.IsActive
	updated.CreatedAt = entry.CreatedAt

	if err := h.DB.Save(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update entry"})
		return
	}

	if err := h.writeAudit(collectorID, updated.ID, "update", before, entrySnapshot(updated)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save audit log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "entry updated", "data": updated})
}

func (h *CollectorHandler) DeactivateEntry(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)
	entryID, err := strconv.Atoi(c.Param("id"))
	if err != nil || entryID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry id"})
		return
	}

	var entry models.DataEntry
	if err := h.DB.Where("id = ? AND collector_id = ?", entryID, collectorID).First(&entry).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}
	if !entry.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry already inactive"})
		return
	}

	before := entrySnapshot(entry)
	entry.IsActive = false
	entry.UpdatedAt = time.Now()

	if err := h.DB.Save(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deactivate entry"})
		return
	}

	if err := h.writeAudit(collectorID, entry.ID, "deactivate", before, entrySnapshot(entry)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save audit log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "entry deactivated", "data": entry})
}

func (h *CollectorHandler) MyEntries(c *gin.Context) {
	collectorID := c.MustGet("user_id").(int)
	var entries []models.DataEntry
	if err := h.DB.
		Preload("Market").
		Preload("Category").
		Preload("Commodity").
		Preload("LocalUnit").
		Preload("StandardUnit").
		Where("collector_id = ?", collectorID).
		Order("created_at DESC").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load entries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

func (h *CollectorHandler) ensureMarketAssignment(collectorID, marketID int) error {
	var assignment models.UserMarketAssignment
	if err := h.DB.Where("user_id = ? AND market_id = ?", collectorID, marketID).First(&assignment).Error; err != nil {
		return fmt.Errorf("collector is not assigned to this market")
	}
	return nil
}

func (h *CollectorHandler) writeAudit(userID, entryID int, action, before, after string) error {
	return h.DB.Create(&models.AuditLog{
		EntryID:   entryID,
		UserID:    userID,
		Action:    action,
		Before:    before,
		After:     after,
		CreatedAt: time.Now(),
	}).Error
}

func validateEntryRequest(req EntryCreateRequest) error {
	return services.ValidateEntry(services.EntryInput{
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
	})
}

func buildEntryFromRequest(req EntryCreateRequest, collectorID int, now time.Time) models.DataEntry {
	converted := services.CalculateConvertedPrice(req.MarketPrice, req.LocalWeightKg, 1.0)
	warning := services.WarningStatus(req.MarketPrice, req.MinimumPrice, req.MaximumPrice)

	return models.DataEntry{
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
		UpdatedAt:        now,
	}
}

func entrySnapshot(entry models.DataEntry) string {
	payload := map[string]interface{}{
		"id":                entry.ID,
		"year":              entry.Year,
		"market_id":         entry.MarketID,
		"collector_id":      entry.CollectorID,
		"category_id":       entry.CategoryID,
		"commodity_id":      entry.CommodityID,
		"brand_type":        entry.BrandType,
		"local_unit_id":     entry.LocalUnitID,
		"local_quantity":    entry.LocalQuantity,
		"local_weight_kg":   entry.LocalWeightKg,
		"standard_unit_id":  entry.StandardUnitID,
		"standard_quantity": entry.StandardQuantity,
		"market_price":      entry.MarketPrice,
		"minimum_price":     entry.MinimumPrice,
		"maximum_price":     entry.MaximumPrice,
		"previous_price":    entry.PreviousPrice,
		"converted_price":   entry.ConvertedPrice,
		"warning_status":    entry.WarningStatus,
		"notes":             entry.Notes,
		"is_active":         entry.IsActive,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}
