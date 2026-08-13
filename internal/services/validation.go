package services

import (
	"fmt"
	"math"
)

type EntryInput struct {
	Year           int
	MarketID       int
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

func ValidateEntry(input EntryInput) error {
	if input.Year < 2020 || input.Year > 2100 {
		return fmt.Errorf("year must be between 2020 and 2100")
	}
	if input.MarketID <= 0 || input.CategoryID <= 0 || input.CommodityID <= 0 {
		return fmt.Errorf("market, category, and commodity are required")
	}
	if input.LocalUnitID <= 0 || input.StandardUnitID <= 0 {
		return fmt.Errorf("unit information is required")
	}
	if input.LocalQuantity <= 0 || input.LocalWeightKg <= 0 {
		return fmt.Errorf("quantity and actual weight must be positive")
	}
	if input.MarketPrice <= 0 {
		return fmt.Errorf("market price must be positive")
	}
	return nil
}

func CalculateConvertedPrice(marketPrice, localWeightKg, standardWeight float64) float64 {
	if localWeightKg <= 0 || standardWeight <= 0 {
		return 0
	}
	return math.Round((marketPrice/localWeightKg)*standardWeight*100) / 100
}

func WarningStatus(marketPrice, minimumPrice, maximumPrice float64) string {
	if minimumPrice > 0 && marketPrice < minimumPrice {
		return "below_minimum"
	}
	if maximumPrice > 0 && marketPrice > maximumPrice {
		return "above_maximum"
	}
	return "normal"
}
