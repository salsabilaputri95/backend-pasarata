package services

import (
	"backend-pasarata/internal/models"
	"fmt"
	"sort"
)

type MarketComparison struct {
	MarketName       string  `json:"market_name"`
	CommodityCode    string  `json:"commodity_code"`
	CommodityName    string  `json:"commodity_name"`
	StandardWeight   string  `json:"standard_weight"`
	StandardUnitName string  `json:"standard_unit_name"`
	CurrentYear      int     `json:"current_year"`
	PreviousYear     int     `json:"previous_year"`
	CurrentAverage   float64 `json:"current_average"`
	PreviousAverage  float64 `json:"previous_average"`
	Delta            float64 `json:"delta"`
	DeltaPercent     float64 `json:"delta_percent"`
}

type MarketSummary struct {
	MarketName       string  `json:"market_name"`
	CommodityCode    string  `json:"commodity_code"`
	CommodityName    string  `json:"commodity_name"`
	StandardWeight   string  `json:"standard_weight"`
	StandardUnitName string  `json:"standard_unit_name"`
	Year             int     `json:"year"`
	AveragePrice     float64 `json:"average_price"`
	MinPrice         float64 `json:"min_price"`
	MaxPrice         float64 `json:"max_price"`
	Count            int     `json:"count"`
}

func getStandardWeightString(entry models.DataEntry) string {
	stdName := "kg"
	if entry.Commodity.StandardUnit != nil && entry.Commodity.StandardUnit.Name != "" {
		stdName = entry.Commodity.StandardUnit.Name
	} else if entry.StandardUnit.Name != "" {
		stdName = entry.StandardUnit.Name
	}

	if entry.LocalWeightKg > 0 {
		return fmt.Sprintf("%g %s", entry.LocalWeightKg, stdName)
	}
	return fmt.Sprintf("1 %s", stdName)
}

func getStandardUnitName(entry models.DataEntry) string {
	if entry.Commodity.StandardUnit != nil && entry.Commodity.StandardUnit.Name != "" {
		return entry.Commodity.StandardUnit.Name
	}
	if entry.StandardUnit.Name != "" {
		return entry.StandardUnit.Name
	}
	return "kg"
}

func BuildMarketComparison(entries []models.DataEntry, currentYear int) []MarketComparison {
	if len(entries) == 0 {
		return nil
	}

	grouped := map[string]struct {
		marketName       string
		commodityCode    string
		commodityName    string
		standardWeight   string
		standardUnitName string
		current          []float64
		previous         []float64
	}{}

	for _, entry := range entries {
		key := fmt.Sprintf("%d:%d", entry.MarketID, entry.CommodityID)
		bucket := grouped[key]
		bucket.marketName = entry.Market.Name
		bucket.commodityCode = entry.Commodity.Code
		bucket.commodityName = entry.Commodity.Name
		bucket.standardWeight = getStandardWeightString(entry)
		bucket.standardUnitName = getStandardUnitName(entry)

		if entry.Year == currentYear {
			bucket.current = append(bucket.current, entry.MarketPrice)
		}
		if entry.Year == currentYear-1 {
			bucket.previous = append(bucket.previous, entry.MarketPrice)
		}

		grouped[key] = bucket
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	comparison := make([]MarketComparison, 0, len(keys))
	for _, key := range keys {
		bucket := grouped[key]
		if len(bucket.current) == 0 {
			continue
		}

		currentAverage := average(bucket.current)
		previousAverage := average(bucket.previous)
		delta := currentAverage - previousAverage
		deltaPercent := 0.0
		if previousAverage > 0 {
			deltaPercent = (delta / previousAverage) * 100
		}

		comparison = append(comparison, MarketComparison{
			MarketName:       bucket.marketName,
			CommodityCode:    bucket.commodityCode,
			CommodityName:    bucket.commodityName,
			StandardWeight:   bucket.standardWeight,
			StandardUnitName: bucket.standardUnitName,
			CurrentYear:      currentYear,
			PreviousYear:     currentYear - 1,
			CurrentAverage:   currentAverage,
			PreviousAverage:  previousAverage,
			Delta:            delta,
			DeltaPercent:     deltaPercent,
		})
	}

	return comparison
}

func BuildMarketSummary(entries []models.DataEntry, year int) []MarketSummary {
	if len(entries) == 0 {
		return nil
	}

	grouped := map[string]struct {
		marketName       string
		commodityCode    string
		commodityName    string
		standardWeight   string
		standardUnitName string
		prices           []float64
	}{}

	for _, entry := range entries {
		if entry.Year != year {
			continue
		}

		key := fmt.Sprintf("%d:%d", entry.MarketID, entry.CommodityID)
		bucket := grouped[key]
		bucket.marketName = entry.Market.Name
		bucket.commodityCode = entry.Commodity.Code
		bucket.commodityName = entry.Commodity.Name
		bucket.standardWeight = getStandardWeightString(entry)
		bucket.standardUnitName = getStandardUnitName(entry)
		bucket.prices = append(bucket.prices, entry.MarketPrice)
		grouped[key] = bucket
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	summary := make([]MarketSummary, 0, len(keys))
	for _, key := range keys {
		bucket := grouped[key]
		if len(bucket.prices) == 0 {
			continue
		}

		avg := average(bucket.prices)
		min := minValue(bucket.prices)
		max := maxValue(bucket.prices)

		summary = append(summary, MarketSummary{
			MarketName:       bucket.marketName,
			CommodityCode:    bucket.commodityCode,
			CommodityName:    bucket.commodityName,
			StandardWeight:   bucket.standardWeight,
			StandardUnitName: bucket.standardUnitName,
			Year:             year,
			AveragePrice:     avg,
			MinPrice:         min,
			MaxPrice:         max,
			Count:            len(bucket.prices),
		})
	}

	return summary
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func minValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}
