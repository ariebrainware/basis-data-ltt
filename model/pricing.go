package model

import "gorm.io/gorm"

// Pricing represents the recorded price for a treatment created by a therapist.
// @Description Pricing information for a treatment
type Pricing struct {
	gorm.Model
	TreatmentID uint  `json:"treatment_id" gorm:"not null;index;uniqueIndex:idx_treatment_pricing" example:"1"`
	TherapistID uint  `json:"therapist_id" gorm:"not null;index" example:"1"`
	Price       int64 `json:"price" gorm:"not null" example:"250000"`
}
