package model

import "gorm.io/gorm"

type Schedule struct {
	gorm.Model
	PatientID   uint   `json:"patient_id" gorm:"not null"`
	TherapistID uint   `json:"therapist_id" gorm:"not null"`
	Day         string `json:"day" gorm:"not null"`
	StartTime   string `json:"start_time" gorm:"not null"`
	EndTime     string `json:"end_time" gorm:"not null"`
}
