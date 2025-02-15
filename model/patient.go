package model

import "gorm.io/gorm"

type Patient struct {
	gorm.Model
	FullName      string `json:"full_name"`
	Password      string `json:"password"`
	Gender        string `json:"gender"`
	Age           int    `json:"age"`
	Job           string `json:"job"`
	Address       string `json:"address"`
	PhoneNumber   string `json:"phone_number"`
	HealthHistory string `json:"health_history"`
}
