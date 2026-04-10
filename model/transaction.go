package model

import "gorm.io/gorm"

// Transaction represents a payment transaction for a treatment.
// @Description Transaction information for a treatment
type Transaction struct {
	gorm.Model
	TreatmentID   uint   `json:"treatment_id" gorm:"not null;index" example:"1"`
	TherapistID   uint   `json:"therapist_id" gorm:"not null;index" example:"1"`
	Amount        int64  `json:"amount" gorm:"not null" example:"50000"`
	Remarks       string `json:"remarks" example:"Urgent handling fee"`
	PaymentMethod string `json:"payment_method" example:"cash"`
	PaymentStatus string `json:"payment_status" gorm:"default:'unpaid'" example:"unpaid"`
}
