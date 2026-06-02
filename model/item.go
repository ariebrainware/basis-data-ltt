package model

import "gorm.io/gorm"

// Item represents a managed inventory item.
// @Description Item information
type Item struct {
	gorm.Model
	Name     string `json:"name" gorm:"not null" example:"Bandage"`
	Quantity int    `json:"quantity" gorm:"not null" example:"100"`
	Price    int64  `json:"price" gorm:"not null" example:"25000"`
}
