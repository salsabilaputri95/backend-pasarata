package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backend-pasarata/internal/models"
	"backend-pasarata/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type importRow struct {
	Year           int
	MarketID       int
	CollectorID    int
	CategoryID     int
	CommodityID    int
	BrandType      string
	LocalUnitID    int
	LocalQuantity  float64
	LocalWeightKg  float64
	StandardUnitID int
	MarketPrice    float64
	MinimumPrice   float64
	MaximumPrice   float64
	PreviousPrice  float64
	Notes          string
}

func (h *AdminHandler) PreviewImportEntries(c *gin.Context) {
	rows, errorsByRow, err := parseImportRows(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	valid := 0
	for i := range rows {
		if len(errorsByRow[i]) == 0 {
			valid++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_rows": len(rows),
		"valid_rows": valid,
		"errors":     errorsByRow,
		"message":    "preview generated",
	})
}

func (h *AdminHandler) ImportEntries(c *gin.Context) {
	rows, errorsByRow, err := parseImportRows(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validRows := make([]importRow, 0, len(rows))
	for i, row := range rows {
		if len(errorsByRow[i]) == 0 {
			validRows = append(validRows, row)
		}
	}
	if len(validRows) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid rows to import", "errors": errorsByRow})
		return
	}

	userID := c.MustGet("user_id").(int)
	now := time.Now()
	imported := 0

	tx := h.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start import transaction"})
		return
	}

	for _, row := range validRows {
		converted := services.CalculateConvertedPrice(row.MarketPrice, row.LocalQuantity, row.LocalWeightKg, 1.0)
		warning := services.WarningStatus(row.MarketPrice, row.MinimumPrice, row.MaximumPrice)

		entry := models.DataEntry{
			Year:             row.Year,
			MarketID:         row.MarketID,
			CollectorID:      row.CollectorID,
			CategoryID:       row.CategoryID,
			CommodityID:      row.CommodityID,
			BrandType:        row.BrandType,
			LocalUnitID:      row.LocalUnitID,
			LocalQuantity:    row.LocalQuantity,
			LocalWeightKg:    row.LocalWeightKg,
			StandardUnitID:   row.StandardUnitID,
			StandardQuantity: row.LocalQuantity,
			MarketPrice:      row.MarketPrice,
			MinimumPrice:     row.MinimumPrice,
			MaximumPrice:     row.MaximumPrice,
			PreviousPrice:    row.PreviousPrice,
			ConvertedPrice:   converted,
			WarningStatus:    warning,
			Notes:            row.Notes,
			IsActive:         true,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Create(&entry).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to import entries"})
			return
		}

		if err := tx.Create(&models.AuditLog{
			EntryID:   entry.ID,
			UserID:    userID,
			Action:    "import",
			Before:    "",
			After:     "entry imported",
			CreatedAt: now,
		}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create import audit log"})
			return
		}

		imported++
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit import"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":         "import completed",
		"imported_rows":   imported,
		"skipped_rows":    len(rows) - imported,
		"validation_logs": errorsByRow,
	})
}

func (h *AdminHandler) ExportReport(c *gin.Context) {
	year, ok := parseYearParam(c)
	if !ok {
		return
	}

	scope := c.DefaultQuery("scope", "summary")
	format := c.DefaultQuery("format", "xlsx")
	entries, err := h.queryReportEntries(c, year, scope)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var content []byte
	var mimeType string
	var ext string

	switch format {
	case "csv":
		content, err = buildCSVContent(scope, year, entries)
		mimeType = "text/csv; charset=utf-8"
		ext = "csv"
	case "xlsx":
		content, err = buildXLSXContent(scope, year, entries)
		mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		ext = "xlsx"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format, use csv or xlsx"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build export file"})
		return
	}

	filename := fmt.Sprintf("pasarata-%s-%d.%s", scope, year, ext)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, mimeType, content)
}

func (h *AdminHandler) queryReportEntries(c *gin.Context, year int, scope string) ([]models.DataEntry, error) {
	marketID := c.Query("market_id")
	collectorID := c.Query("collector_id")
	warningStatus := c.Query("warning_status")

	query := h.DB.Where("year = ?", year)
	if marketID != "" {
		query = query.Where("market_id = ?", marketID)
	}
	if collectorID != "" {
		query = query.Where("collector_id = ?", collectorID)
	}
	if warningStatus != "" {
		query = query.Where("warning_status = ?", warningStatus)
	}

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
		return nil, fmt.Errorf("invalid scope, use entries|summary|comparison")
	}

	var entries []models.DataEntry
	if err := query.Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to query report data")
	}
	return entries, nil
}

func parseImportRows(c *gin.Context) ([]importRow, map[int][]string, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, nil, fmt.Errorf("file is required")
	}

	headers, dataRows, err := readTabularFile(fileHeader)
	if err != nil {
		return nil, nil, err
	}
	if len(headers) == 0 {
		return nil, nil, fmt.Errorf("missing header row")
	}

	headerIndex := map[string]int{}
	for i, h := range headers {
		headerIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}

	defaultCollectorID := 0
	if raw := strings.TrimSpace(c.PostForm("collector_id_default")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr == nil && parsed > 0 {
			defaultCollectorID = parsed
		}
	}

	rows := make([]importRow, 0, len(dataRows))
	errorsByRow := map[int][]string{}
	for i, data := range dataRows {
		rowNo := i + 2
		row := importRow{}
		rowErrors := make([]string, 0)

		row.Year, rowErrors = parseIntCell(data, headerIndex, "year", rowNo, rowErrors)
		row.MarketID, rowErrors = parseIntCell(data, headerIndex, "market_id", rowNo, rowErrors)
		row.CategoryID, rowErrors = parseIntCell(data, headerIndex, "category_id", rowNo, rowErrors)
		row.CommodityID, rowErrors = parseIntCell(data, headerIndex, "commodity_id", rowNo, rowErrors)
		row.LocalUnitID, rowErrors = parseIntCell(data, headerIndex, "local_unit_id", rowNo, rowErrors)
		row.LocalQuantity, rowErrors = parseFloatCell(data, headerIndex, "local_quantity", rowNo, rowErrors)
		row.LocalWeightKg, rowErrors = parseFloatCell(data, headerIndex, "local_weight_kg", rowNo, rowErrors)
		row.StandardUnitID, rowErrors = parseIntCell(data, headerIndex, "standard_unit_id", rowNo, rowErrors)
		row.MarketPrice, rowErrors = parseFloatCell(data, headerIndex, "market_price", rowNo, rowErrors)
		row.MinimumPrice, rowErrors = parseFloatCell(data, headerIndex, "minimum_price", rowNo, rowErrors)
		row.MaximumPrice, rowErrors = parseFloatCell(data, headerIndex, "maximum_price", rowNo, rowErrors)
		row.PreviousPrice = parseOptionalFloatCell(data, headerIndex, "previous_price")
		row.BrandType = parseOptionalStringCell(data, headerIndex, "brand_type")
		row.Notes = parseOptionalStringCell(data, headerIndex, "notes")
		if hasColumn(headerIndex, "collector_id") {
			row.CollectorID, rowErrors = parseIntCell(data, headerIndex, "collector_id", rowNo, rowErrors)
		} else if defaultCollectorID > 0 {
			row.CollectorID = defaultCollectorID
		} else {
			rowErrors = append(rowErrors, "collector_id required (column or collector_id_default)")
		}

		if err := services.ValidateEntry(services.EntryInput{
			Year:           row.Year,
			MarketID:       row.MarketID,
			CategoryID:     row.CategoryID,
			CommodityID:    row.CommodityID,
			BrandType:      row.BrandType,
			LocalUnitID:    row.LocalUnitID,
			LocalQuantity:  row.LocalQuantity,
			LocalWeightKg:  row.LocalWeightKg,
			StandardUnitID: row.StandardUnitID,
			MarketPrice:    row.MarketPrice,
			MinimumPrice:   row.MinimumPrice,
			MaximumPrice:   row.MaximumPrice,
			PreviousPrice:  row.PreviousPrice,
			Notes:          row.Notes,
		}); err != nil {
			rowErrors = append(rowErrors, err.Error())
		}

		rows = append(rows, row)
		errorsByRow[rowNo] = rowErrors
	}

	return rows, errorsByRow, nil
}

func readTabularFile(fileHeader *multipart.FileHeader) ([]string, [][]string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file")
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".csv":
		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err != nil {
			return nil, nil, fmt.Errorf("invalid csv file")
		}
		if len(records) == 0 {
			return nil, nil, nil
		}
		return records[0], records[1:], nil
	case ".xlsx":
		content, err := io.ReadAll(file)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read xlsx")
		}
		xls, err := excelize.OpenReader(bytes.NewReader(content))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid xlsx file")
		}
		defer xls.Close()

		sheets := xls.GetSheetList()
		if len(sheets) == 0 {
			return nil, nil, nil
		}
		rows, err := xls.GetRows(sheets[0])
		if err != nil || len(rows) == 0 {
			return nil, nil, fmt.Errorf("failed to read xlsx rows")
		}
		return rows[0], rows[1:], nil
	default:
		return nil, nil, fmt.Errorf("unsupported file type, use .csv or .xlsx")
	}
}

func parseIntCell(row []string, index map[string]int, name string, rowNo int, errs []string) (int, []string) {
	value, ok := getCell(row, index, name)
	if !ok || strings.TrimSpace(value) == "" {
		return 0, append(errs, fmt.Sprintf("row %d: %s is required", rowNo, name))
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, append(errs, fmt.Sprintf("row %d: %s must be integer", rowNo, name))
	}
	return parsed, errs
}

func parseFloatCell(row []string, index map[string]int, name string, rowNo int, errs []string) (float64, []string) {
	value, ok := getCell(row, index, name)
	if !ok || strings.TrimSpace(value) == "" {
		return 0, append(errs, fmt.Sprintf("row %d: %s is required", rowNo, name))
	}
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, append(errs, fmt.Sprintf("row %d: %s must be number", rowNo, name))
	}
	return parsed, errs
}

func parseOptionalFloatCell(row []string, index map[string]int, name string) float64 {
	value, ok := getCell(row, index, name)
	if !ok || strings.TrimSpace(value) == "" {
		return 0
	}
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseOptionalStringCell(row []string, index map[string]int, name string) string {
	value, ok := getCell(row, index, name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func hasColumn(index map[string]int, name string) bool {
	_, ok := index[strings.ToLower(name)]
	return ok
}

func getCell(row []string, index map[string]int, name string) (string, bool) {
	idx, ok := index[strings.ToLower(name)]
	if !ok || idx >= len(row) {
		return "", false
	}
	return row[idx], true
}

func buildXLSXContent(scope string, year int, entries []models.DataEntry) ([]byte, error) {
	xls := excelize.NewFile()
	sheet := "Report"
	xls.SetSheetName("Sheet1", sheet)

	rows := [][]string{}
	switch scope {
	case "summary":
		rows = append(rows, []string{"year", "market", "commodity", "average_price", "min_price", "max_price", "count"})
		for _, row := range services.BuildMarketSummary(entries, year) {
			rows = append(rows, []string{
				strconv.Itoa(row.Year),
				row.MarketName,
				row.CommodityName,
				strconv.FormatFloat(row.AveragePrice, 'f', 2, 64),
				strconv.FormatFloat(row.MinPrice, 'f', 2, 64),
				strconv.FormatFloat(row.MaxPrice, 'f', 2, 64),
				strconv.Itoa(row.Count),
			})
		}
	case "comparison":
		rows = append(rows, []string{"current_year", "previous_year", "market", "commodity", "current_average", "previous_average", "delta", "delta_percent"})
		for _, row := range services.BuildMarketComparison(entries, year) {
			rows = append(rows, []string{
				strconv.Itoa(row.CurrentYear),
				strconv.Itoa(row.PreviousYear),
				row.MarketName,
				row.CommodityName,
				strconv.FormatFloat(row.CurrentAverage, 'f', 2, 64),
				strconv.FormatFloat(row.PreviousAverage, 'f', 2, 64),
				strconv.FormatFloat(row.Delta, 'f', 2, 64),
				strconv.FormatFloat(row.DeltaPercent, 'f', 2, 64),
			})
		}
	case "entries":
		rows = append(rows, []string{"id", "year", "market", "collector", "category", "commodity", "market_price", "minimum_price", "maximum_price", "warning_status", "created_at", "notes"})
		for _, row := range entries {
			rows = append(rows, []string{
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
			})
		}
	default:
		return nil, fmt.Errorf("invalid scope")
	}

	for r, row := range rows {
		for c, value := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := xls.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}

	buffer, err := xls.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
