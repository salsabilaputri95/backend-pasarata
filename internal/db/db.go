package db

import (
	"fmt"

	"backend-pasarata/internal/config"
	"backend-pasarata/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Market{},
		&models.CommodityCategory{},
		&models.Commodity{},
		&models.Unit{},
		&models.UserMarketAssignment{},
		&models.DataEntry{},
		&models.AuditLog{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate models: %w", err)
	}

	return db, nil
}
