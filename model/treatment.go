package model

import (
	"gorm.io/gorm"
)

type Treatment struct {
	gorm.Model
	TreatmentDate string `json:"treatment_date" gorm:"not null"`
	PatientCode   string `json:"patient_code" gorm:"not null;unique"`
	TherapistID   uint   `json:"therapist_id" gorm:"not null"`
	Issues        string `json:"issues" gorm:"not null"`
	Treatment     string `json:"treatment" gorm:"not null"`
	Remarks       string `json:"remarks"`
	NextVisit     string `json:"next_visit" gorm:"not null"`
}

type TreatementRequest struct {
	TreatmentDate string   `json:"treatment_date" binding:"required"`
	PatientCode   string   `json:"patient_code" binding:"required"`
	TherapistID   uint     `json:"therapist_id" binding:"required"`
	Issues        string   `json:"issues" binding:"required"`
	Treatment     []string `json:"treatment" binding:"required"`
	Remarks       string   `json:"remarks"`
	NextVisit     string   `json:"next_visit" binding:"required"`
}
