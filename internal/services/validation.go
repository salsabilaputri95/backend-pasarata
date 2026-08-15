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

// CalculateConvertedPrice — Rumus Konversi 3 Pilar:
// Harga Standar = (Harga Pasar / (Kuantitas Lokal * Bobot Aktual kg)) * Bobot Standar
//
// 3 Pilar Variabel Utama (sesuai Pasara'ta System Concept Overview):
// 1. Nilai harga pasar (marketPrice) — nilai transaksi yang dibayar
// 2. Bobot atau isi aktual (localWeightKg) — berat isi bersih per satuan lokal dalam kg
// 3. Kuantitas atau pembagian satuan (localQuantity) — jumlah unit/satuan lokal yang dibeli
//
// Satuan Standar:
// Bobot acuan standar komoditas (standardWeight, default 1.0 kg = 1.000 gram).
//
// Contoh Konsep:
// Transaksi pasar: Rp10.000
// Bobot aktual: 0,89 kg (890 gram)
// Kuantitas: 1 satuan
// Satuan standar: 1 kg (1.000 gram)
// Perhitungan: Rp10.000 ÷ (1 × 0,89) × 1 = Rp11.235,96 / kg
func CalculateConvertedPrice(marketPrice, localQuantity, localWeightKg, standardWeight float64) float64 {
	if localQuantity <= 0 {
		localQuantity = 1.0
	}
	totalWeight := localQuantity * localWeightKg
	if totalWeight <= 0 || standardWeight <= 0 || marketPrice <= 0 {
		return 0
	}
	return math.Round((marketPrice/totalWeight)*standardWeight*100) / 100
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
