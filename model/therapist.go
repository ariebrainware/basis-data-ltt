package model

import "gorm.io/gorm"

type Therapist struct {
	gorm.Model
	FullName    string `json:"full_name" gorm:"column:full_name"`
	Email       string `json:"email" gorm:"column:email"`
	Password    string `json:"password" gorm:"column:password"`
	PhoneNumber string `json:"phone_number" gorm:"column:phone_number"`
	Address     string `json:"address" gorm:"column:address"`
	DateOfBirth string `json:"date_of_birth" gorm:"column:date_of_birth"`
	NIK         string `json:"nik" gorm:"column:nik"`
	Weight      int    `json:"weight" gorm:"column:weight"`
	Height      int    `json:"height" gorm:"column:height"`
	Role        string `json:"role" gorm:"column:role"`
	IsApproved  bool   `json:"is_approved" gorm:"column:is_approved;default:false"`
}
