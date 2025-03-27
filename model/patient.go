package model

import "gorm.io/gorm"

type Patient struct {
	gorm.Model
	FullName       string `json:"full_name" gorm:"column:full_name"`
	Password       string `json:"password" gorm:"column:password"`
	Gender         string `json:"gender" gorm:"column:gender"`
	Age            int    `json:"age" gorm:"column:age"`
	Job            string `json:"job" gorm:"column:job"`
	Address        string `json:"address" gorm:"column:address"`
	PhoneNumber    string `json:"phone_number" gorm:"column:phone_number"`
	HealthHistory  string `json:"health_history" gorm:"column:health_history"`
	SurgeryHistory string `json:"surgery_history" gorm:"column:surgery_history"`
	PatientCode    string `json:"patient_code" gorm:"column:patient_code"`
}
