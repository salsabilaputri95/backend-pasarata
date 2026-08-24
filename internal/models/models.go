package models

import (
	"time"

	"gorm.io/gorm"
)

type Role string

type UserStatus string

const (
	RoleAdmin     Role = "admin"
	RoleCollector Role = "collector"

	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

type User struct {
	ID           int            `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"not null" json:"-"`
	FullName     string         `gorm:"not null" json:"full_name"`
	Role         Role           `gorm:"type:varchar(20);not null;default:'collector'" json:"role"`
	Status       UserStatus     `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Market struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Province  string    `gorm:"not null" json:"province"`
	District  string    `gorm:"not null" json:"district"`
	NKS       string    `gorm:"not null" json:"nks"`
	Name      string    `gorm:"not null" json:"name"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CommodityCategory struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"not null;unique" json:"name"`
	Type      string    `gorm:"not null" json:"type"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Commodity struct {
	ID             int               `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string            `gorm:"not null;unique" json:"code"`
	Name           string            `gorm:"not null" json:"name"`
	CategoryID     int               `gorm:"not null" json:"category_id"`
	Category       CommodityCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	StandardUnitID *int              `json:"standard_unit_id,omitempty"`
	StandardUnit   *Unit             `gorm:"foreignKey:StandardUnitID" json:"standard_unit,omitempty"`
	BrandType      string            `gorm:"type:varchar(100)" json:"brand_type,omitempty"`
	Active         bool              `gorm:"default:true" json:"active"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Unit struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"not null" json:"name"`
	IsStandard       bool      `gorm:"default:false" json:"is_standard"`
	StandardValue    float64   `gorm:"type:decimal(18,4);default:1" json:"standard_value"`
	StandardUnitName string    `gorm:"type:varchar(50);default:''" json:"standard_unit_name"`
	ConversionFactor float64   `gorm:"type:decimal(18,8);default:1" json:"conversion_factor"`
	Active           bool      `gorm:"default:true" json:"active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type UserMarketAssignment struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"not null" json:"user_id"`
	MarketID  int       `gorm:"not null" json:"market_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Market    Market    `gorm:"foreignKey:MarketID" json:"market,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type DataEntry struct {
	ID               int               `gorm:"primaryKey;autoIncrement" json:"id"`
	Year             int               `gorm:"not null" json:"year"`
	MarketID         int               `gorm:"not null" json:"market_id"`
	Market           Market            `gorm:"foreignKey:MarketID" json:"market,omitempty"`
	CollectorID      int               `gorm:"not null" json:"collector_id"`
	Collector        User              `gorm:"foreignKey:CollectorID" json:"collector,omitempty"`
	CategoryID       int               `gorm:"not null" json:"category_id"`
	Category         CommodityCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	CommodityID      int               `gorm:"not null" json:"commodity_id"`
	Commodity        Commodity         `gorm:"foreignKey:CommodityID" json:"commodity,omitempty"`
	BrandType        string            `gorm:"type:varchar(200)" json:"brand_type,omitempty"`
	LocalUnitID      int               `gorm:"not null" json:"local_unit_id"`
	LocalUnit        Unit              `gorm:"foreignKey:LocalUnitID" json:"local_unit,omitempty"`
	LocalQuantity    float64           `gorm:"type:decimal(18,4)" json:"local_quantity"`
	LocalWeightKg    float64           `gorm:"type:decimal(18,4)" json:"local_weight_kg"`
	StandardUnitID   int               `gorm:"not null" json:"standard_unit_id"`
	StandardUnit     Unit              `gorm:"foreignKey:StandardUnitID" json:"standard_unit,omitempty"`
	StandardQuantity float64           `gorm:"type:decimal(18,4)" json:"standard_quantity"`
	MarketPrice      float64           `gorm:"type:decimal(18,2)" json:"market_price"`
	MinimumPrice     float64           `gorm:"type:decimal(18,2)" json:"minimum_price"`
	MaximumPrice     float64           `gorm:"type:decimal(18,2)" json:"maximum_price"`
	PreviousPrice    float64           `gorm:"type:decimal(18,2);default:0" json:"previous_price"`
	ConvertedPrice   float64           `gorm:"type:decimal(18,2);default:0" json:"converted_price"`
	WarningStatus    string            `gorm:"type:varchar(30);default:'normal'" json:"warning_status"`
	Notes            string            `gorm:"type:text" json:"notes,omitempty"`
	IsActive         bool              `gorm:"default:true" json:"is_active"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type AuditLog struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	EntryID   int       `gorm:"not null" json:"entry_id"`
	UserID    int       `gorm:"not null" json:"user_id"`
	Action    string    `gorm:"not null" json:"action"`
	Before    string    `gorm:"type:text" json:"before,omitempty"`
	After     string    `gorm:"type:text" json:"after,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
