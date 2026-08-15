package handlers

import (
	"net/http"
	"strconv"
	"time"

	"backend-pasarata/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetEntriesFiltered — GET /api/admin/entries
// Query params: year, market_id, collector_id, warning_status, is_active
func (h *AdminHandler) GetEntriesFiltered(c *gin.Context) {
	query := h.DB.
		Preload("Market").
		Preload("Collector").
		Preload("Category").
		Preload("Commodity").
		Preload("LocalUnit").
		Preload("StandardUnit").
		Order("created_at DESC")

	if yearStr := c.Query("year"); yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil || year < 2020 || year > 2100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return
		}
		query = query.Where("year = ?", year)
	}

	if marketIDStr := c.Query("market_id"); marketIDStr != "" {
		marketID, err := strconv.Atoi(marketIDStr)
		if err != nil || marketID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market_id"})
			return
		}
		query = query.Where("market_id = ?", marketID)
	}

	if collectorIDStr := c.Query("collector_id"); collectorIDStr != "" {
		collectorID, err := strconv.Atoi(collectorIDStr)
		if err != nil || collectorID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collector_id"})
			return
		}
		query = query.Where("collector_id = ?", collectorID)
	}

	if warningStatus := c.Query("warning_status"); warningStatus != "" {
		valid := map[string]bool{"normal": true, "below_minimum": true, "above_maximum": true}
		if !valid[warningStatus] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid warning_status"})
			return
		}
		query = query.Where("warning_status = ?", warningStatus)
	}

	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		switch isActiveStr {
		case "true":
			query = query.Where("is_active = ?", true)
		case "false":
			query = query.Where("is_active = ?", false)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "is_active harus 'true' atau 'false'"})
			return
		}
	}

	var entries []models.DataEntry
	if err := query.Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// AdminUpdateEntry — PUT /api/admin/entries/:id
// Admin mengedit entri milik siapa pun; hitung ulang konversi & warning.
func (h *AdminHandler) AdminUpdateEntry(c *gin.Context) {
	adminID := c.MustGet("user_id").(int)

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

	if err := validateEntryRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var entry models.DataEntry
	if err := h.DB.First(&entry, entryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load entry"})
		}
		return
	}

	before := entrySnapshot(entry)
	now := time.Now()

	// Pertahankan collector_id asli dan waktu buat; admin tidak mengambil alih kepemilikan
	updated := buildEntryFromRequest(req, entry.CollectorID, now)
	updated.ID = entry.ID
	updated.IsActive = entry.IsActive
	updated.CreatedAt = entry.CreatedAt

	if err := h.DB.Save(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update entry"})
		return
	}

	// Audit log — simpan before/after lengkap
	_ = h.writeAdminAudit(adminID, updated.ID, "update", before, entrySnapshot(updated))

	c.JSON(http.StatusOK, gin.H{"message": "entry updated", "data": updated})
}

// AdminDeleteEntry — DELETE /api/admin/entries/:id
// Hard delete — hanya Admin. Audit dicatat sebelum hapus.
func (h *AdminHandler) AdminDeleteEntry(c *gin.Context) {
	adminID := c.MustGet("user_id").(int)

	entryID, err := strconv.Atoi(c.Param("id"))
	if err != nil || entryID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry id"})
		return
	}

	var entry models.DataEntry
	if err := h.DB.First(&entry, entryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load entry"})
		}
		return
	}

	before := entrySnapshot(entry)

	// Catat audit SEBELUM hapus agar entry_id masih ada saat dicatat
	_ = h.writeAdminAudit(adminID, entry.ID, "delete", before, "")

	if err := h.DB.Unscoped().Delete(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete entry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "entry deleted"})
}

// writeAdminAudit menulis audit log untuk aksi admin.
func (h *AdminHandler) writeAdminAudit(userID, entryID int, action, before, after string) error {
	return h.DB.Create(&models.AuditLog{
		EntryID:   entryID,
		UserID:    userID,
		Action:    action,
		Before:    before,
		After:     after,
		CreatedAt: time.Now(),
	}).Error
}
