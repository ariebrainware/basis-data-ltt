package model

import "gorm.io/gorm"

// Pricing represents the recorded price for a therapist
// @Description Pricing information for a treatment
type Pricing struct {
	gorm.Model
	TherapistID uint   `json:"therapist_id" gorm:"not null;index" example:"1"`
	Price       int64  `json:"price" gorm:"not null" example:"250000"`
	Commission  string `json:"commission" gorm:"column:commission;type:varchar(200)" example:"50000"`
	Description string `json:"description" gorm:"type:text" example:""`
}
