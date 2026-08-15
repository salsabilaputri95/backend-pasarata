package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend-pasarata/internal/models"
	"backend-pasarata/internal/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminHandler struct {
	DB *gorm.DB
}

type CreateCollectorRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
}

type CreateMarketRequest struct {
	Province string `json:"province" binding:"required"`
	District string `json:"district" binding:"required"`
	NKS      string `json:"nks" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}

type CreateCommodityRequest struct {
	Code       string `json:"code" binding:"required"`
	Name       string `json:"name" binding:"required"`
	CategoryID int    `json:"category_id" binding:"required"`
	BrandType  string `json:"brand_type"`
}

type CreateUnitRequest struct {
	Name             string  `json:"name" binding:"required"`
	IsStandard       bool    `json:"is_standard"`
	ConversionFactor float64 `json:"conversion_factor"`
}

type CreateAssignmentRequest struct {
	UserID   int `json:"user_id" binding:"required"`
	MarketID int `json:"market_id" binding:"required"`
}

type YearBreakdown struct {
	Year           int   `json:"year"`
	TotalEntries   int64 `json:"total_entries"`
	WarningEntries int64 `json:"warning_entries"`
}

type MarketBreakdown struct {
	MarketID       int    `json:"market_id"`
	MarketName     string `json:"market_name"`
	District       string `json:"district"`
	TotalEntries   int64  `json:"total_entries"`
	WarningEntries int64  `json:"warning_entries"`
}

type CollectorBreakdown struct {
	CollectorID    int    `json:"collector_id"`
	CollectorName  string `json:"collector_name"`
	Username       string `json:"username"`
	TotalEntries   int64  `json:"total_entries"`
	WarningEntries int64  `json:"warning_entries"`
}

func (h *AdminHandler) Dashboard(c *gin.Context) {
	var collectors int64
	var markets int64
	var commodityCount int64
	var totalEntries int64
	var warningEntries int64

	_ = h.DB.Model(&models.User{}).Where("role = ?", models.RoleCollector).Count(&collectors).Error
	_ = h.DB.Model(&models.Market{}).Count(&markets).Error
	_ = h.DB.Model(&models.Commodity{}).Count(&commodityCount).Error
	_ = h.DB.Model(&models.DataEntry{}).Count(&totalEntries).Error
	_ = h.DB.Model(&models.DataEntry{}).Where("warning_status != ?", "normal").Count(&warningEntries).Error

	// Breakdown per Tahun
	var byYear []YearBreakdown
	_ = h.DB.Model(&models.DataEntry{}).
		Select("year, count(*) as total_entries, sum(case when warning_status != 'normal' then 1 else 0 end) as warning_entries").
		Group("year").
		Order("year DESC").
		Scan(&byYear).Error

	// Breakdown per Pasar
	var byMarket []MarketBreakdown
	_ = h.DB.Table("data_entries").
		Select("data_entries.market_id, markets.name as market_name, markets.district, count(*) as total_entries, sum(case when data_entries.warning_status != 'normal' then 1 else 0 end) as warning_entries").
		Joins("LEFT JOIN markets ON markets.id = data_entries.market_id").
		Group("data_entries.market_id, markets.name, markets.district").
		Order("total_entries DESC").
		Scan(&byMarket).Error

	// Breakdown per Pendata
	var byCollector []CollectorBreakdown
	_ = h.DB.Table("data_entries").
		Select("data_entries.collector_id, users.full_name as collector_name, users.username, count(*) as total_entries, sum(case when data_entries.warning_status != 'normal' then 1 else 0 end) as warning_entries").
		Joins("LEFT JOIN users ON users.id = data_entries.collector_id").
		Group("data_entries.collector_id, users.full_name, users.username").
		Order("total_entries DESC").
		Scan(&byCollector).Error

	// 10 Entri Terbaru
	var recentEntries []models.DataEntry
	_ = h.DB.Preload("Market").
		Preload("Collector").
		Preload("Category").
		Preload("Commodity").
		Preload("LocalUnit").
		Preload("StandardUnit").
		Order("created_at DESC").
		Limit(10).
		Find(&recentEntries).Error

	c.JSON(http.StatusOK, gin.H{
		"collectors":      collectors,
		"markets":         markets,
		"commodities":     commodityCount,
		"total_entries":   totalEntries,
		"warning_entries": warningEntries,
		"by_year":         byYear,
		"by_market":       byMarket,
		"by_collector":    byCollector,
		"recent_entries":  recentEntries,
		"message":         "admin dashboard",
	})
}


func (h *AdminHandler) GetCollectors(c *gin.Context) {
	var users []models.User
	if err := h.DB.Where("role = ?", models.RoleCollector).Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load collectors"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

func (h *AdminHandler) CreateCollector(c *gin.Context) {
	var req CreateCollectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var existing models.User
	if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		FullName:     req.FullName,
		Role:         models.RoleCollector,
		Status:       models.UserStatusActive,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create collector"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "collector created", "data": user})
}

func (h *AdminHandler) GetMarkets(c *gin.Context) {
	var markets []models.Market
	if err := h.DB.Order("created_at DESC").Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load markets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": markets})
}

func (h *AdminHandler) CreateMarket(c *gin.Context) {
	var req CreateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market payload"})
		return
	}

	market := models.Market{
		Province: req.Province,
		District: req.District,
		NKS:      req.NKS,
		Name:     req.Name,
		Active:   true,
	}

	if err := h.DB.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create market"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "market created", "data": market})
}

func (h *AdminHandler) GetCategories(c *gin.Context) {
	var categories []models.CommodityCategory
	if err := h.DB.Order("created_at DESC").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

func (h *AdminHandler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category payload"})
		return
	}

	category := models.CommodityCategory{
		Name:   req.Name,
		Type:   req.Type,
		Active: true,
	}

	if err := h.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "category created", "data": category})
}

func (h *AdminHandler) GetCommodities(c *gin.Context) {
	var commodities []models.Commodity
	if err := h.DB.Preload("Category").Order("created_at DESC").Find(&commodities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load commodities"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": commodities})
}

func (h *AdminHandler) CreateCommodity(c *gin.Context) {
	var req CreateCommodityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid commodity payload"})
		return
	}

	commodity := models.Commodity{
		Code:       req.Code,
		Name:       req.Name,
		CategoryID: req.CategoryID,
		BrandType:  req.BrandType,
		Active:     true,
	}

	if err := h.DB.Create(&commodity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create commodity"})
		return
	}

	_ = h.DB.Preload("Category").First(&commodity, commodity.ID)
	c.JSON(http.StatusCreated, gin.H{"message": "commodity created", "data": commodity})
}

func (h *AdminHandler) GetUnits(c *gin.Context) {
	var units []models.Unit
	if err := h.DB.Order("created_at DESC").Find(&units).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load units"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": units})
}

func (h *AdminHandler) CreateUnit(c *gin.Context) {
	var req CreateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid unit payload"})
		return
	}

	unit := models.Unit{
		Name:             req.Name,
		IsStandard:       req.IsStandard,
		ConversionFactor: req.ConversionFactor,
		Active:           true,
	}

	if err := h.DB.Create(&unit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create unit"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "unit created", "data": unit})
}

func (h *AdminHandler) GetAssignments(c *gin.Context) {
	var assignments []models.UserMarketAssignment
	if err := h.DB.Preload("User").Preload("Market").Order("created_at DESC").Find(&assignments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load assignments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": assignments})
}

func (h *AdminHandler) CreateAssignment(c *gin.Context) {
	var req CreateAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assignment payload"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collector not found"})
		return
	}
	var market models.Market
	if err := h.DB.First(&market, req.MarketID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "market not found"})
		return
	}

	var existing models.UserMarketAssignment
	if err := h.DB.Where("user_id = ? AND market_id = ?", req.UserID, req.MarketID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "assignment already exists"})
		return
	}

	assignment := models.UserMarketAssignment{UserID: req.UserID, MarketID: req.MarketID}
	if err := h.DB.Create(&assignment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create assignment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "assignment created", "data": assignment})
}

func (h *AdminHandler) GetEntries(c *gin.Context) {
	var entries []models.DataEntry
	if err := h.DB.
		Preload("Market").
		Preload("Collector").
		Preload("Category").
		Preload("Commodity").
		Preload("LocalUnit").
		Preload("StandardUnit").
		Order("created_at DESC").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load entries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

func (h *AdminHandler) GetAuditLogs(c *gin.Context) {
	var logs []models.AuditLog
	if err := h.DB.
		Preload("User").
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *AdminHandler) GetComparison(c *gin.Context) {
	yearQuery := c.DefaultQuery("year", strconv.Itoa(time.Now().Year()))
	year, err := strconv.Atoi(yearQuery)
	if err != nil || year < 2020 || year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
		return
	}

	var entries []models.DataEntry
	if err := h.DB.
		Preload("Market").
		Preload("Commodity").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load comparison data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"year": year,
		"data": services.BuildMarketComparison(entries, year),
	})
}

func (h *AdminHandler) GetSummary(c *gin.Context) {
	yearQuery := c.DefaultQuery("year", strconv.Itoa(time.Now().Year()))
	year, err := strconv.Atoi(yearQuery)
	if err != nil || year < 2020 || year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
		return
	}

	var entries []models.DataEntry
	if err := h.DB.
		Preload("Market").
		Preload("Commodity").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load summary data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"year": year,
		"data": services.BuildMarketSummary(entries, year),
	})
}

func (h *AdminHandler) ExportCSV(c *gin.Context) {
	year, ok := parseYearParam(c)
	if !ok {
		return
	}

	scope := c.DefaultQuery("scope", "summary")
	var entries []models.DataEntry
	query := h.DB.Where("year = ?", year)

	switch scope {
	case "entries":
		query = query.
			Preload("Market").
			Preload("Collector").
			Preload("Category").
			Preload("Commodity").
			Preload("LocalUnit").
			Preload("StandardUnit")
	case "summary", "comparison":
		query = query.
			Preload("Market").
			Preload("Commodity")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope, use entries|summary|comparison"})
		return
	}

	if err := query.Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load export data"})
		return
	}

	content, err := buildCSVContent(scope, year, entries)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build csv"})
		return
	}

	filename := fmt.Sprintf("pasarata-%s-%d.csv", scope, year)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", content)
}

func parseYearParam(c *gin.Context) (int, bool) {
	yearQuery := c.DefaultQuery("year", strconv.Itoa(time.Now().Year()))
	year, err := strconv.Atoi(yearQuery)
	if err != nil || year < 2020 || year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
		return 0, false
	}
	return year, true
}

func buildCSVContent(scope string, year int, entries []models.DataEntry) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("\ufeff")
	writer := csv.NewWriter(&builder)

	switch scope {
	case "summary":
		if err := writer.Write([]string{"year", "market", "commodity", "average_price", "min_price", "max_price", "count"}); err != nil {
			return nil, err
		}
		for _, row := range services.BuildMarketSummary(entries, year) {
			record := []string{
				strconv.Itoa(row.Year),
				row.MarketName,
				row.CommodityName,
				strconv.FormatFloat(row.AveragePrice, 'f', 2, 64),
				strconv.FormatFloat(row.MinPrice, 'f', 2, 64),
				strconv.FormatFloat(row.MaxPrice, 'f', 2, 64),
				strconv.Itoa(row.Count),
			}
			if err := writer.Write(record); err != nil {
				return nil, err
			}
		}
	case "comparison":
		if err := writer.Write([]string{"current_year", "previous_year", "market", "commodity", "current_average", "previous_average", "delta", "delta_percent"}); err != nil {
			return nil, err
		}
		for _, row := range services.BuildMarketComparison(entries, year) {
			record := []string{
				strconv.Itoa(row.CurrentYear),
				strconv.Itoa(row.PreviousYear),
				row.MarketName,
				row.CommodityName,
				strconv.FormatFloat(row.CurrentAverage, 'f', 2, 64),
				strconv.FormatFloat(row.PreviousAverage, 'f', 2, 64),
				strconv.FormatFloat(row.Delta, 'f', 2, 64),
				strconv.FormatFloat(row.DeltaPercent, 'f', 2, 64),
			}
			if err := writer.Write(record); err != nil {
				return nil, err
			}
		}
	case "entries":
		if err := writer.Write([]string{"id", "year", "market", "collector", "category", "commodity", "market_price", "minimum_price", "maximum_price", "warning_status", "created_at", "notes"}); err != nil {
			return nil, err
		}
		for _, row := range entries {
			record := []string{
				strconv.Itoa(row.ID),
				strconv.Itoa(row.Year),
				row.Market.Name,
				row.Collector.FullName,
				row.Category.Name,
				row.Commodity.Name,
				strconv.FormatFloat(row.MarketPrice, 'f', 2, 64),
				strconv.FormatFloat(row.MinimumPrice, 'f', 2, 64),
				strconv.FormatFloat(row.MaximumPrice, 'f', 2, 64),
				row.WarningStatus,
				row.CreatedAt.Format(time.RFC3339),
				row.Notes,
			}
			if err := writer.Write(record); err != nil {
				return nil, err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(builder.String()), nil
}
func (h *AdminHandler) UpdateMarket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CreateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var market models.Market
	if err := h.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "market not found"})
		return
	}
	market.Province = req.Province
	market.District = req.District
	market.NKS = req.NKS
	market.Name = req.Name
	if err := h.DB.Save(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update market"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "market updated", "data": market})
}

func (h *AdminHandler) SetMarketStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var market models.Market
	if err := h.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "market not found"})
		return
	}
	market.Active = body.Active
	if err := h.DB.Save(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update market status"})
		return
	}
	status := "nonaktif"
	if body.Active {
		status = "aktif"
	}
	c.JSON(http.StatusOK, gin.H{"message": "market " + status, "data": market})
}

func (h *AdminHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var category models.CommodityCategory
	if err := h.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	category.Name = req.Name
	category.Type = req.Type
	if err := h.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "category updated", "data": category})
}

func (h *AdminHandler) SetCategoryStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var category models.CommodityCategory
	if err := h.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	category.Active = body.Active
	if err := h.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update category status"})
		return
	}
	status := "nonaktif"
	if body.Active {
		status = "aktif"
	}
	c.JSON(http.StatusOK, gin.H{"message": "category " + status, "data": category})
}

func (h *AdminHandler) UpdateCommodity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CreateCommodityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var commodity models.Commodity
	if err := h.DB.First(&commodity, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "commodity not found"})
		return
	}
	commodity.Code = req.Code
	commodity.Name = req.Name
	commodity.CategoryID = req.CategoryID
	commodity.BrandType = req.BrandType
	if err := h.DB.Save(&commodity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update commodity"})
		return
	}
	_ = h.DB.Preload("Category").First(&commodity, commodity.ID)
	c.JSON(http.StatusOK, gin.H{"message": "commodity updated", "data": commodity})
}

func (h *AdminHandler) SetCommodityStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var commodity models.Commodity
	if err := h.DB.First(&commodity, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "commodity not found"})
		return
	}
	commodity.Active = body.Active
	if err := h.DB.Save(&commodity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update commodity status"})
		return
	}
	status := "nonaktif"
	if body.Active {
		status = "aktif"
	}
	c.JSON(http.StatusOK, gin.H{"message": "commodity " + status, "data": commodity})
}

func (h *AdminHandler) UpdateUnit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CreateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var unit models.Unit
	if err := h.DB.First(&unit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit not found"})
		return
	}
	unit.Name = req.Name
	unit.IsStandard = req.IsStandard
	unit.ConversionFactor = req.ConversionFactor
	if err := h.DB.Save(&unit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update unit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unit updated", "data": unit})
}

func (h *AdminHandler) SetUnitStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var unit models.Unit
	if err := h.DB.First(&unit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit not found"})
		return
	}
	unit.Active = body.Active
	if err := h.DB.Save(&unit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update unit status"})
		return
	}
	status := "nonaktif"
	if body.Active {
		status = "aktif"
	}
	c.JSON(http.StatusOK, gin.H{"message": "unit " + status, "data": unit})
}

// DeleteAssignment — DELETE /api/admin/assignments/:id
// Setelah dihapus, Pendata tidak bisa lagi input ke pasar tersebut.
func (h *AdminHandler) DeleteAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var assignment models.UserMarketAssignment
	if err := h.DB.First(&assignment, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}
	if err := h.DB.Delete(&assignment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete assignment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "assignment deleted"})
}
