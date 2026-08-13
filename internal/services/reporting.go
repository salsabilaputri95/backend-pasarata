package services

import (
	"backend-pasarata/internal/models"
	"fmt"
	"sort"
)

type MarketComparison struct {
	MarketName      string
	CommodityName   string
	CurrentYear     int
	PreviousYear    int
	CurrentAverage  float64
	PreviousAverage float64
	Delta           float64
	DeltaPercent    float64
}

type MarketSummary struct {
	MarketName    string
	CommodityName string
	Year          int
	AveragePrice  float64
	MinPrice      float64
	MaxPrice      float64
	Count         int
}

func BuildMarketComparison(entries []models.DataEntry, currentYear int) []MarketComparison {
	if len(entries) == 0 {
		return nil
	}

	grouped := map[string]struct {
		marketName    string
		commodityName string
		current       []float64
		previous      []float64
	}{}

	for _, entry := range entries {
		key := fmt.Sprintf("%d:%d", entry.MarketID, entry.CommodityID)
		bucket := grouped[key]
		bucket.marketName = entry.Market.Name
		bucket.commodityName = entry.Commodity.Name

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
			MarketName:      bucket.marketName,
			CommodityName:   bucket.commodityName,
			CurrentYear:     currentYear,
			PreviousYear:    currentYear - 1,
			CurrentAverage:  currentAverage,
			PreviousAverage: previousAverage,
			Delta:           delta,
			DeltaPercent:    deltaPercent,
		})
	}

	return comparison
}

func BuildMarketSummary(entries []models.DataEntry, year int) []MarketSummary {
	if len(entries) == 0 {
		return nil
	}

	grouped := map[string]struct {
		marketName    string
		commodityName string
		prices        []float64
	}{}

	for _, entry := range entries {
		if entry.Year != year {
			continue
		}

		key := fmt.Sprintf("%d:%d", entry.MarketID, entry.CommodityID)
		bucket := grouped[key]
		bucket.marketName = entry.Market.Name
		bucket.commodityName = entry.Commodity.Name
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
			MarketName:    bucket.marketName,
			CommodityName: bucket.commodityName,
			Year:          year,
			AveragePrice:  avg,
			MinPrice:      min,
			MaxPrice:      max,
			Count:         len(bucket.prices),
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
