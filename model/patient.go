package model

import "gorm.io/gorm"

// Patient represents a patient entity
// @Description Patient information
type Patient struct {
	gorm.Model
	FullName       string `json:"full_name" gorm:"column:full_name" example:"John Doe"`
	Password       string `json:"password" gorm:"column:password" example:"hashed_password"`
	Gender         string `json:"gender" gorm:"column:gender" example:"Male"`
	Age            int    `json:"age" gorm:"column:age" example:"30"`
	Job            string `json:"job" gorm:"column:job" example:"Engineer"`
	Address        string `json:"address" gorm:"column:address" example:"123 Main St"`
	Email          string `json:"email" gorm:"column:email" example:"john@example.com"`
	PhoneNumber    string `json:"phone_number" gorm:"column:phone_number" example:"081234567890"`
	HealthHistory  string `json:"health_history" gorm:"column:health_history" example:"Diabetes,Hypertension"`
	SurgeryHistory string `json:"surgery_history" gorm:"column:surgery_history" example:"Appendectomy 2020"`
	PatientCode    string `json:"patient_code" gorm:"column:patient_code" example:"J001"`
}

type UpdatePatientRequest struct {
	FullName       string `json:"full_name" example:"John Doe"`
	Password       string `json:"password" example:"hashed_password"`
	Gender         string `json:"gender" example:"Male"`
	Age            int    `json:"age" example:"30"`
	Job            string `json:"job" example:"Engineer"`
	Address        string `json:"address" example:"123 Main St"`
	Email          string `json:"email" example:"john@example.com"`
	PhoneNumber    string `json:"phone_number" example:"081234567890"`
	HealthHistory  string `json:"health_history" example:"Diabetes,Hypertension"`
	SurgeryHistory string `json:"surgery_history" example:"Appendectomy 2020"`
	PatientCode    string `json:"patient_code" example:"J001"`
}
