package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
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

func (h *AdminHandler) InspectImportHeaders(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	headers, dataRows, err := readTabularFile(fileHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file kosong atau tidak memiliki baris header"})
		return
	}

	sampleCount := 3
	if len(dataRows) < sampleCount {
		sampleCount = len(dataRows)
	}

	c.JSON(http.StatusOK, gin.H{
		"headers":     headers,
		"sample_rows": dataRows[:sampleCount],
		"total_rows":  len(dataRows),
		"message":     "headers inspected",
	})
}

func (h *AdminHandler) PreviewImportEntries(c *gin.Context) {
	rows, errorsByRow, sampleParsed, err := parseImportRows(c)
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
		"total_rows":    len(rows),
		"valid_rows":    valid,
		"errors":        errorsByRow,
		"sample_parsed": sampleParsed,
		"message":       "preview generated",
	})
}

func (h *AdminHandler) ImportEntries(c *gin.Context) {
	rows, errorsByRow, _, err := parseImportRows(c)
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

func parseImportRows(c *gin.Context) ([]importRow, map[int][]string, []importRow, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("file is required")
	}

	headers, dataRows, err := readTabularFile(fileHeader)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(headers) == 0 {
		return nil, nil, nil, fmt.Errorf("missing header row")
	}

	headerIndex := map[string]int{}
	for i, h := range headers {
		headerIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}

	// Parse optional custom column mapping (e.g. {"market_price": "Harga Pasar", "commodity_id": "Kode"})
	var mapping map[string]string
	if rawMapping := strings.TrimSpace(c.PostForm("mapping")); rawMapping != "" {
		_ = json.Unmarshal([]byte(rawMapping), &mapping)
	}
	if mapping == nil {
		mapping = make(map[string]string)
	}

	// Parse optional default values (e.g. {"year": "2026", "collector_id": "2", "local_quantity": "1"})
	var defaults map[string]string
	if rawDefaults := strings.TrimSpace(c.PostForm("defaults")); rawDefaults != "" {
		_ = json.Unmarshal([]byte(rawDefaults), &defaults)
	}
	if defaults == nil {
		defaults = make(map[string]string)
	}

	// Legacy backward compatibility for collector_id_default
	if raw := strings.TrimSpace(c.PostForm("collector_id_default")); raw != "" {
		if _, exists := defaults["collector_id"]; !exists {
			defaults["collector_id"] = raw
		}
	}

	rows := make([]importRow, 0, len(dataRows))
	errorsByRow := map[int][]string{}
	sampleParsed := make([]importRow, 0, 5)

	for i, data := range dataRows {
		rowNo := i + 2
		row := importRow{}
		rowErrors := make([]string, 0)

		row.Year, rowErrors = parseMappedIntCell(data, headerIndex, "year", mapping, defaults, rowNo, rowErrors)
		row.MarketID, rowErrors = parseMappedIntCell(data, headerIndex, "market_id", mapping, defaults, rowNo, rowErrors)
		row.CollectorID, rowErrors = parseMappedIntCell(data, headerIndex, "collector_id", mapping, defaults, rowNo, rowErrors)
		row.CategoryID, rowErrors = parseMappedIntCell(data, headerIndex, "category_id", mapping, defaults, rowNo, rowErrors)
		row.CommodityID, rowErrors = parseMappedIntCell(data, headerIndex, "commodity_id", mapping, defaults, rowNo, rowErrors)
		row.BrandType = parseMappedStringCell(data, headerIndex, "brand_type", mapping, defaults)
		row.LocalUnitID, rowErrors = parseMappedIntCell(data, headerIndex, "local_unit_id", mapping, defaults, rowNo, rowErrors)
		row.LocalQuantity, rowErrors = parseMappedFloatCell(data, headerIndex, "local_quantity", mapping, defaults, rowNo, rowErrors, 1.0)
		row.LocalWeightKg, rowErrors = parseMappedFloatCell(data, headerIndex, "local_weight_kg", mapping, defaults, rowNo, rowErrors, 1.0)
		row.StandardUnitID, rowErrors = parseMappedIntCell(data, headerIndex, "standard_unit_id", mapping, defaults, rowNo, rowErrors)
		row.MarketPrice, rowErrors = parseMappedFloatCell(data, headerIndex, "market_price", mapping, defaults, rowNo, rowErrors, 0)
		row.MinimumPrice, rowErrors = parseMappedFloatCell(data, headerIndex, "minimum_price", mapping, defaults, rowNo, rowErrors, 0)
		row.MaximumPrice, rowErrors = parseMappedFloatCell(data, headerIndex, "maximum_price", mapping, defaults, rowNo, rowErrors, 0)
		row.PreviousPrice = parseMappedOptionalFloatCell(data, headerIndex, "previous_price", mapping, defaults)
		row.Notes = parseMappedStringCell(data, headerIndex, "notes", mapping, defaults)

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
		if len(rowErrors) > 0 {
			errorsByRow[rowNo] = rowErrors
		}
		if len(sampleParsed) < 5 {
			sampleParsed = append(sampleParsed, row)
		}
	}

	return rows, errorsByRow, sampleParsed, nil
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

func resolveMappedCell(row []string, index map[string]int, fieldName string, mapping map[string]string, defaults map[string]string) (string, bool) {
	// 1. Cek apakah ada mapping khusus untuk field ini
	if mappedHeader, ok := mapping[fieldName]; ok && strings.TrimSpace(mappedHeader) != "" {
		if cellVal, found := getCell(row, index, mappedHeader); found && strings.TrimSpace(cellVal) != "" {
			return strings.TrimSpace(cellVal), true
		}
	}

	// 2. Cek apakah nama field cocok langsung dengan nama kolom (auto-match)
	if cellVal, found := getCell(row, index, fieldName); found && strings.TrimSpace(cellVal) != "" {
		return strings.TrimSpace(cellVal), true
	}

	// 3. Cek apakah ada nilai default yang ditentukan user
	if defVal, ok := defaults[fieldName]; ok && strings.TrimSpace(defVal) != "" {
		return strings.TrimSpace(defVal), true
	}

	return "", false
}

func parseMappedIntCell(row []string, index map[string]int, name string, mapping map[string]string, defaults map[string]string, rowNo int, errs []string) (int, []string) {
	value, ok := resolveMappedCell(row, index, name, mapping, defaults)
	if !ok || strings.TrimSpace(value) == "" {
		return 0, append(errs, fmt.Sprintf("row %d: %s is required", rowNo, name))
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, append(errs, fmt.Sprintf("row %d: %s must be integer", rowNo, name))
	}
	return parsed, errs
}

func parseMappedFloatCell(row []string, index map[string]int, name string, mapping map[string]string, defaults map[string]string, rowNo int, errs []string, fallback float64) (float64, []string) {
	value, ok := resolveMappedCell(row, index, name, mapping, defaults)
	if !ok || strings.TrimSpace(value) == "" {
		if fallback > 0 {
			return fallback, errs
		}
		return 0, append(errs, fmt.Sprintf("row %d: %s is required", rowNo, name))
	}
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		if fallback > 0 {
			return fallback, errs
		}
		return 0, append(errs, fmt.Sprintf("row %d: %s must be number", rowNo, name))
	}
	return parsed, errs
}

func parseMappedOptionalFloatCell(row []string, index map[string]int, name string, mapping map[string]string, defaults map[string]string) float64 {
	value, ok := resolveMappedCell(row, index, name, mapping, defaults)
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

func parseMappedStringCell(row []string, index map[string]int, name string, mapping map[string]string, defaults map[string]string) string {
	value, ok := resolveMappedCell(row, index, name, mapping, defaults)
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
	idx, ok := index[strings.ToLower(strings.TrimSpace(name))]
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
		rows = append(rows, []string{"year", "market", "commodity_id", "commodity", "standard_weight", "average_price", "min_price", "max_price", "count"})
		for _, row := range services.BuildMarketSummary(entries, year) {
			rows = append(rows, []string{
				strconv.Itoa(row.Year),
				row.MarketName,
				row.CommodityCode,
				row.CommodityName,
				row.StandardWeight,
				strconv.FormatFloat(row.AveragePrice, 'f', 2, 64),
				strconv.FormatFloat(row.MinPrice, 'f', 2, 64),
				strconv.FormatFloat(row.MaxPrice, 'f', 2, 64),
				strconv.Itoa(row.Count),
			})
		}
	case "comparison":
		rows = append(rows, []string{"current_year", "previous_year", "market", "commodity_id", "commodity", "standard_weight", "current_average", "previous_average", "delta", "delta_percent"})
		for _, row := range services.BuildMarketComparison(entries, year) {
			rows = append(rows, []string{
				strconv.Itoa(row.CurrentYear),
				strconv.Itoa(row.PreviousYear),
				row.MarketName,
				row.CommodityCode,
				row.CommodityName,
				row.StandardWeight,
				strconv.FormatFloat(row.CurrentAverage, 'f', 2, 64),
				strconv.FormatFloat(row.PreviousAverage, 'f', 2, 64),
				strconv.FormatFloat(row.Delta, 'f', 2, 64),
				strconv.FormatFloat(row.DeltaPercent, 'f', 2, 64),
			})
		}
	case "entries":
		rows = append(rows, []string{"id", "year", "market", "collector", "category", "commodity_id", "commodity", "market_price", "minimum_price", "maximum_price", "warning_status", "created_at", "notes"})
		for _, row := range entries {
			commodityID := row.Commodity.Code
			if commodityID == "" {
				commodityID = strconv.Itoa(row.CommodityID)
			}
			rows = append(rows, []string{
				strconv.Itoa(row.ID),
				strconv.Itoa(row.Year),
				row.Market.Name,
				row.Collector.FullName,
				row.Category.Name,
				commodityID,
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

// ── MASTER DATA IMPORT (STEP-09) ──────────────────────────────────────────

type MasterCommodityRow struct {
	Code         string
	Name         string
	CategoryID   int
	CategoryName string
	BrandType    string
	IsUpdate     bool
}

type MasterCategoryRow struct {
	Name     string
	Type     string
	IsUpdate bool
}

type MasterUnitRow struct {
	Name             string
	IsStandard       bool
	ConversionFactor float64
	IsUpdate         bool
}

type MasterMarketRow struct {
	Province string
	District string
	NKS      string
	Name     string
	IsUpdate bool
}

// PreviewImportMaster — POST /api/admin/import/master/preview
func (h *AdminHandler) PreviewImportMaster(c *gin.Context) {
	target := strings.ToLower(strings.TrimSpace(c.DefaultPostForm("target", "commodities")))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	headers, dataRows, err := readTabularFile(fileHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing header row"})
		return
	}

	headerIndex := map[string]int{}
	for i, hName := range headers {
		headerIndex[strings.ToLower(strings.TrimSpace(hName))] = i
	}

	var mapping map[string]string
	if rawMapping := strings.TrimSpace(c.PostForm("mapping")); rawMapping != "" {
		_ = json.Unmarshal([]byte(rawMapping), &mapping)
	}
	if mapping == nil {
		mapping = make(map[string]string)
	}

	var defaults map[string]string
	if rawDefaults := strings.TrimSpace(c.PostForm("defaults")); rawDefaults != "" {
		_ = json.Unmarshal([]byte(rawDefaults), &defaults)
	}
	if defaults == nil {
		defaults = make(map[string]string)
	}

	errorsByRow := map[int][]string{}
	var validCount, newCount, updateCount int
	var sampleParsed []interface{}

	switch target {
	case "commodities":
		// Cache existing categories and commodity codes
		existingCategories := make(map[string]int)
		var cats []models.CommodityCategory
		h.DB.Find(&cats)
		for _, cat := range cats {
			existingCategories[strings.ToLower(cat.Name)] = cat.ID
		}

		existingCodes := make(map[string]int)
		var comms []models.Commodity
		h.DB.Find(&comms)
		for _, comm := range comms {
			existingCodes[strings.ToLower(comm.Code)] = comm.ID
		}

		for i, data := range dataRows {
			rowNo := i + 2
			rowErrs := make([]string, 0)

			code, ok := resolveMappedCell(data, headerIndex, "code", mapping, defaults)
			if !ok || strings.TrimSpace(code) == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: code is required", rowNo))
			}

			name, ok := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !ok || strings.TrimSpace(name) == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: name is required", rowNo))
			}

			catID := 0
			catName, _ := resolveMappedCell(data, headerIndex, "category_name", mapping, defaults)
			catIDStr, hasCatID := resolveMappedCell(data, headerIndex, "category_id", mapping, defaults)
			if hasCatID && catIDStr != "" {
				parsed, parseErr := strconv.Atoi(catIDStr)
				if parseErr == nil && parsed > 0 {
					catID = parsed
				}
			}
			if catID == 0 && catName != "" {
				if id, found := existingCategories[strings.ToLower(catName)]; found {
					catID = id
				}
			}
			if catID == 0 && catName == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: category_id or category_name is required", rowNo))
			}

			brandType, _ := resolveMappedCell(data, headerIndex, "brand_type", mapping, defaults)

			isUpdate := false
			if code != "" {
				if _, found := existingCodes[strings.ToLower(code)]; found {
					isUpdate = true
					updateCount++
				} else {
					newCount++
				}
			}

			if len(rowErrs) == 0 {
				validCount++
			} else {
				errorsByRow[rowNo] = rowErrs
			}

			if len(sampleParsed) < 5 {
				sampleParsed = append(sampleParsed, gin.H{
					"code":          code,
					"name":          name,
					"category_id":   catID,
					"category_name": catName,
					"brand_type":    brandType,
					"is_update":     isUpdate,
				})
			}
		}

	case "categories":
		existingCats := make(map[string]int)
		var cats []models.CommodityCategory
		h.DB.Find(&cats)
		for _, cItem := range cats {
			existingCats[strings.ToLower(cItem.Name)] = cItem.ID
		}

		for i, data := range dataRows {
			rowNo := i + 2
			rowErrs := make([]string, 0)

			name, ok := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !ok || strings.TrimSpace(name) == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: name is required", rowNo))
			}

			catType, _ := resolveMappedCell(data, headerIndex, "type", mapping, defaults)
			if catType == "" {
				catType = "Makanan"
			}

			isUpdate := false
			if name != "" {
				if _, found := existingCats[strings.ToLower(name)]; found {
					isUpdate = true
					updateCount++
				} else {
					newCount++
				}
			}

			if len(rowErrs) == 0 {
				validCount++
			} else {
				errorsByRow[rowNo] = rowErrs
			}

			if len(sampleParsed) < 5 {
				sampleParsed = append(sampleParsed, gin.H{
					"name":      name,
					"type":      catType,
					"is_update": isUpdate,
				})
			}
		}

	case "units":
		existingUnits := make(map[string]int)
		var units []models.Unit
		h.DB.Find(&units)
		for _, uItem := range units {
			existingUnits[strings.ToLower(uItem.Name)] = uItem.ID
		}

		for i, data := range dataRows {
			rowNo := i + 2
			rowErrs := make([]string, 0)

			name, ok := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !ok || strings.TrimSpace(name) == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: name is required", rowNo))
			}

			isStdStr, _ := resolveMappedCell(data, headerIndex, "is_standard", mapping, defaults)
			isStd := strings.EqualFold(isStdStr, "true") || isStdStr == "1" || strings.EqualFold(isStdStr, "ya")

			convStr, _ := resolveMappedCell(data, headerIndex, "conversion_factor", mapping, defaults)
			convFactor := 1.0
			if convStr != "" {
				parsed, parseErr := strconv.ParseFloat(strings.ReplaceAll(convStr, ",", "."), 64)
				if parseErr == nil && parsed > 0 {
					convFactor = parsed
				}
			}

			isUpdate := false
			if name != "" {
				if _, found := existingUnits[strings.ToLower(name)]; found {
					isUpdate = true
					updateCount++
				} else {
					newCount++
				}
			}

			if len(rowErrs) == 0 {
				validCount++
			} else {
				errorsByRow[rowNo] = rowErrs
			}

			if len(sampleParsed) < 5 {
				sampleParsed = append(sampleParsed, gin.H{
					"name":              name,
					"is_standard":       isStd,
					"conversion_factor": convFactor,
					"is_update":         isUpdate,
				})
			}
		}

	case "markets":
		existingMarkets := make(map[string]int)
		var markets []models.Market
		h.DB.Find(&markets)
		for _, mItem := range markets {
			existingMarkets[strings.ToLower(mItem.NKS)] = mItem.ID
		}

		for i, data := range dataRows {
			rowNo := i + 2
			rowErrs := make([]string, 0)

			name, ok := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !ok || strings.TrimSpace(name) == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: name is required", rowNo))
			}
			nks, ok := resolveMappedCell(data, headerIndex, "nks", mapping, defaults)
			if !ok || strings.TrimSpace(nks) == "" {
				rowErrs = append(rowErrs, fmt.Sprintf("row %d: nks is required", rowNo))
			}
			prov, _ := resolveMappedCell(data, headerIndex, "province", mapping, defaults)
			district, _ := resolveMappedCell(data, headerIndex, "district", mapping, defaults)

			isUpdate := false
			if nks != "" {
				if _, found := existingMarkets[strings.ToLower(nks)]; found {
					isUpdate = true
					updateCount++
				} else {
					newCount++
				}
			}

			if len(rowErrs) == 0 {
				validCount++
			} else {
				errorsByRow[rowNo] = rowErrs
			}

			if len(sampleParsed) < 5 {
				sampleParsed = append(sampleParsed, gin.H{
					"name":      name,
					"nks":       nks,
					"province":  prov,
					"district":  district,
					"is_update": isUpdate,
				})
			}
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "target master tidak valid (gunakan: commodities, categories, units, markets)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"target":        target,
		"total_rows":    len(dataRows),
		"valid_rows":    validCount,
		"new_rows":      newCount,
		"update_rows":   updateCount,
		"errors":        errorsByRow,
		"sample_parsed": sampleParsed,
		"message":       "master preview generated",
	})
}

// ImportMaster — POST /api/admin/import/master/commit
func (h *AdminHandler) ImportMaster(c *gin.Context) {
	target := strings.ToLower(strings.TrimSpace(c.DefaultPostForm("target", "commodities")))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	headers, dataRows, err := readTabularFile(fileHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing header row"})
		return
	}

	headerIndex := map[string]int{}
	for i, hName := range headers {
		headerIndex[strings.ToLower(strings.TrimSpace(hName))] = i
	}

	var mapping map[string]string
	if rawMapping := strings.TrimSpace(c.PostForm("mapping")); rawMapping != "" {
		_ = json.Unmarshal([]byte(rawMapping), &mapping)
	}
	if mapping == nil {
		mapping = make(map[string]string)
	}

	var defaults map[string]string
	if rawDefaults := strings.TrimSpace(c.PostForm("defaults")); rawDefaults != "" {
		_ = json.Unmarshal([]byte(rawDefaults), &defaults)
	}
	if defaults == nil {
		defaults = make(map[string]string)
	}

	userID := c.MustGet("user_id").(int)
	now := time.Now()
	var createdCount, updatedCount, skippedCount int

	tx := h.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
		return
	}

	switch target {
	case "commodities":
		for _, data := range dataRows {
			code, okCode := resolveMappedCell(data, headerIndex, "code", mapping, defaults)
			name, okName := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !okCode || !okName || code == "" || name == "" {
				skippedCount++
				continue
			}

			catID := 0
			catName, _ := resolveMappedCell(data, headerIndex, "category_name", mapping, defaults)
			catIDStr, hasCatID := resolveMappedCell(data, headerIndex, "category_id", mapping, defaults)
			if hasCatID && catIDStr != "" {
				parsed, parseErr := strconv.Atoi(catIDStr)
				if parseErr == nil && parsed > 0 {
					catID = parsed
				}
			}
			if catID == 0 && catName != "" {
				var cat models.CommodityCategory
				if err := tx.Where("LOWER(name) = ?", strings.ToLower(catName)).First(&cat).Error; err == nil {
					catID = cat.ID
				} else {
					// Auto-create category if missing
					newCat := models.CommodityCategory{Name: catName, Type: "Makanan", Active: true, CreatedAt: now, UpdatedAt: now}
					if err := tx.Create(&newCat).Error; err == nil {
						catID = newCat.ID
					}
				}
			}
			if catID == 0 {
				skippedCount++
				continue
			}

			brandType, _ := resolveMappedCell(data, headerIndex, "brand_type", mapping, defaults)

			var existing models.Commodity
			if err := tx.Where("LOWER(code) = ?", strings.ToLower(code)).First(&existing).Error; err == nil {
				// Upsert: update existing
				existing.Name = name
				existing.CategoryID = catID
				existing.BrandType = brandType
				existing.Active = true
				existing.UpdatedAt = now
				_ = tx.Save(&existing)
				updatedCount++
			} else {
				// Create new
				newComm := models.Commodity{
					Code:       code,
					Name:       name,
					CategoryID: catID,
					BrandType:  brandType,
					Active:     true,
					CreatedAt:  now,
					UpdatedAt:  now,
				}
				_ = tx.Create(&newComm)
				createdCount++
			}
		}

	case "categories":
		for _, data := range dataRows {
			name, ok := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !ok || strings.TrimSpace(name) == "" {
				skippedCount++
				continue
			}
			catType, _ := resolveMappedCell(data, headerIndex, "type", mapping, defaults)
			if catType == "" {
				catType = "Makanan"
			}

			var existing models.CommodityCategory
			if err := tx.Where("LOWER(name) = ?", strings.ToLower(name)).First(&existing).Error; err == nil {
				existing.Type = catType
				existing.Active = true
				existing.UpdatedAt = now
				_ = tx.Save(&existing)
				updatedCount++
			} else {
				newCat := models.CommodityCategory{Name: name, Type: catType, Active: true, CreatedAt: now, UpdatedAt: now}
				_ = tx.Create(&newCat)
				createdCount++
			}
		}

	case "units":
		for _, data := range dataRows {
			name, ok := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			if !ok || strings.TrimSpace(name) == "" {
				skippedCount++
				continue
			}
			isStdStr, _ := resolveMappedCell(data, headerIndex, "is_standard", mapping, defaults)
			isStd := strings.EqualFold(isStdStr, "true") || isStdStr == "1" || strings.EqualFold(isStdStr, "ya")
			convStr, _ := resolveMappedCell(data, headerIndex, "conversion_factor", mapping, defaults)
			convFactor := 1.0
			if convStr != "" {
				parsed, parseErr := strconv.ParseFloat(strings.ReplaceAll(convStr, ",", "."), 64)
				if parseErr == nil && parsed > 0 {
					convFactor = parsed
				}
			}

			var existing models.Unit
			if err := tx.Where("LOWER(name) = ?", strings.ToLower(name)).First(&existing).Error; err == nil {
				existing.IsStandard = isStd
				existing.ConversionFactor = convFactor
				existing.Active = true
				existing.UpdatedAt = now
				_ = tx.Save(&existing)
				updatedCount++
			} else {
				newUnit := models.Unit{Name: name, IsStandard: isStd, ConversionFactor: convFactor, Active: true, CreatedAt: now, UpdatedAt: now}
				_ = tx.Create(&newUnit)
				createdCount++
			}
		}

	case "markets":
		for _, data := range dataRows {
			name, okName := resolveMappedCell(data, headerIndex, "name", mapping, defaults)
			nks, okNks := resolveMappedCell(data, headerIndex, "nks", mapping, defaults)
			if !okName || !okNks || name == "" || nks == "" {
				skippedCount++
				continue
			}
			prov, _ := resolveMappedCell(data, headerIndex, "province", mapping, defaults)
			district, _ := resolveMappedCell(data, headerIndex, "district", mapping, defaults)

			var existing models.Market
			if err := tx.Where("LOWER(nks) = ?", strings.ToLower(nks)).First(&existing).Error; err == nil {
				existing.Name = name
				existing.Province = prov
				existing.District = district
				existing.Active = true
				existing.UpdatedAt = now
				_ = tx.Save(&existing)
				updatedCount++
			} else {
				newMarket := models.Market{Name: name, NKS: nks, Province: prov, District: district, Active: true, CreatedAt: now, UpdatedAt: now}
				_ = tx.Create(&newMarket)
				createdCount++
			}
		}
	}

	_ = tx.Create(&models.AuditLog{
		UserID:    userID,
		Action:    "import_master",
		Before:    "",
		After:     fmt.Sprintf("master target: %s (created: %d, updated: %d)", target, createdCount, updatedCount),
		CreatedAt: now,
	})

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit master import"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "master import completed",
		"target":        target,
		"created_count": createdCount,
		"updated_count": updatedCount,
		"skipped_count": skippedCount,
	})
}

