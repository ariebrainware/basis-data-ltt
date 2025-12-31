package model

import (
	"gorm.io/gorm"
)

// Treatment represents a treatment entity
// @Description Treatment information
type Treatment struct {
	gorm.Model
	ID            uint   `json:"id" example:"1"`
	TreatmentDate string `json:"treatment_date" gorm:"not null" example:"2025-01-15"`
	PatientCode   string `json:"patient_code" gorm:"not null" example:"J001"`
	TherapistID   uint   `json:"therapist_id" gorm:"not null" example:"1"`
	Issues        string `json:"issues" gorm:"not null" example:"Back pain"`
	Treatment     string `json:"treatment" gorm:"not null" example:"Massage therapy,Exercise"`
	Remarks       string `json:"remarks" example:"Patient showed improvement"`
	NextVisit     string `json:"next_visit" gorm:"not null" example:"2025-01-22"`
}

// TreatementRequest represents a treatment request
// @Description Treatment request information
type TreatementRequest struct {
	TreatmentDate string   `json:"treatment_date" example:"2025-01-15"`
	PatientCode   string   `json:"patient_code" example:"J001"`
	TherapistID   uint     `json:"therapist_id" example:"1"`
	Issues        string   `json:"issues" example:"Back pain"`
	Treatment     []string `json:"treatment,omitempty" example:"Massage therapy,Exercise"`
	Remarks       string   `json:"remarks,omitempty" example:"Patient showed improvement"`
	NextVisit     string   `json:"next_visit,omitempty" example:"2025-01-22"`
}

// ListTreatementResponse represents a treatment list response
// @Description Treatment list response information
type ListTreatementResponse struct {
	Treatment
	TherapistName string `json:"therapist_name" gorm:"column:therapist_name" example:"Dr. John Smith"`
	PatientName   string `json:"patient_name" gorm:"column:patient_name" example:"John Doe"`
	Age           int    `json:"age" gorm:"column:age" example:"30"`
}
