package model

import (
	"gorm.io/gorm"
)

type Treatment struct {
	gorm.Model
	TreatmentDate string `json:"treatment_date" gorm:"not null"`
	PatientCode   string `json:"patient_code" gorm:"not null"`
	TherapistID   uint   `json:"therapist_id" gorm:"not null"`
	Issues        string `json:"issues" gorm:"not null"`
	Treatment     string `json:"treatment" gorm:"not null"`
	Remarks       string `json:"remarks"`
	NextVisit     string `json:"next_visit" gorm:"not null"`
}

type TreatementRequest struct {
	TreatmentDate string   `json:"treatment_date"`
	PatientCode   string   `json:"patient_code"`
	TherapistID   uint     `json:"therapist_id"`
	Issues        string   `json:"issues"`
	Treatment     []string `json:"treatment,omitempty"`
	Remarks       string   `json:"remarks,omitempty"`
	NextVisit     string   `json:"next_visit,omitempty"`
}

type ListTreatementResponse struct {
	Treatment
	TherapistName string `json:"therapist_name" gorm:"column:therapist_name"`
	PatientName   string `json:"patient_name" gorm:"column:patient_name"`
	Age           int    `json:"age" gorm:"column:age"`
}
