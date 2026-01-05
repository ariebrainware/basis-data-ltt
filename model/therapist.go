package model

import "gorm.io/gorm"

// Therapist represents a therapist entity
// @Description Therapist information
type Therapist struct {
	gorm.Model
	FullName    string `json:"full_name" gorm:"column:full_name" example:"Dr. John Smith"`
	Email       string `json:"email" gorm:"column:email" example:"dr.john@example.com"`
	Password    string `json:"password" gorm:"column:password" example:"hashed_password"`
	PhoneNumber string `json:"phone_number" gorm:"column:phone_number" example:"081234567890"`
	Address     string `json:"address" gorm:"column:address" example:"123 Main St"`
	DateOfBirth string `json:"date_of_birth" gorm:"column:date_of_birth" example:"1980-01-01"`
	NIK         string `json:"nik" gorm:"column:nik" example:"1234567890123456"`
	Weight      int    `json:"weight" gorm:"column:weight" example:"70"`
	Height      int    `json:"height" gorm:"column:height" example:"175"`
	Role        string `json:"role" gorm:"column:role" example:"Physical Therapist"`
	IsApproved  bool   `json:"is_approved" gorm:"column:is_approved;default:false" example:"false"`
}
