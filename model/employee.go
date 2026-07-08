package model

import "gorm.io/gorm"

// Employee represents an employee entity
// @Description Employee information
type Employee struct {
	gorm.Model
	NIK         int64  `json:"nik" gorm:"column:nik;uniqueIndex;not null" example:"1234567890123456"`
	FullName    string `json:"fullname" gorm:"column:fullname;not null" example:"John Doe"`
	Gender      string `json:"gender" gorm:"column:gender;not null" example:"Male"`
	Address     string `json:"address" gorm:"column:address;not null" example:"123 Main St"`
	Religion    string `json:"religion" gorm:"column:religion;not null" example:"Islam"`
	PhoneNumber string `json:"phone_number" gorm:"column:phone_number;not null" example:"081234567890"`
	Email       string `json:"email" gorm:"column:email;not null" example:"john.doe@example.com"`
	JoinedDate  string `json:"joined_date" gorm:"column:joined_date;not null" example:"2025-01-15"`
	Position    string `json:"position" gorm:"column:position;not null" example:"Manager"`
	BaseSalary  int    `json:"base_salary" gorm:"column:base_salary;not null" example:"5000000"`
	LunchMoney  int    `json:"lunch_money" gorm:"column:lunch_money;not null" example:"50000"`
}

// CreateEmployeeRequest represents the employee creation request body
// @Description Employee creation request payload
type CreateEmployeeRequest struct {
	NIK         int64  `json:"nik" binding:"required" example:"1234567890123456"`
	FullName    string `json:"fullname" binding:"required" example:"John Doe"`
	Gender      string `json:"gender" binding:"required" example:"Male"`
	Address     string `json:"address" binding:"required" example:"123 Main St"`
	Religion    string `json:"religion" binding:"required" example:"Islam"`
	PhoneNumber string `json:"phone_number" binding:"required" example:"081234567890"`
	Email       string `json:"email" binding:"required,email" example:"john.doe@example.com"`
	JoinedDate  string `json:"joined_date" binding:"required" example:"2025-01-15"`
	Position    string `json:"position" binding:"required" example:"Manager"`
	BaseSalary  int    `json:"base_salary" binding:"required" example:"5000000"`
	LunchMoney  int    `json:"lunch_money" binding:"required" example:"50000"`
}

// UpdateEmployeeRequest represents the employee update request body
// @Description Employee update request payload
type UpdateEmployeeRequest struct {
	NIK         *int64  `json:"nik" example:"1234567890123456"`
	FullName    string  `json:"fullname" example:"John Doe"`
	Gender      string  `json:"gender" example:"Male"`
	Address     string  `json:"address" example:"123 Main St"`
	Religion    string  `json:"religion" example:"Islam"`
	PhoneNumber string  `json:"phone_number" example:"081234567890"`
	Email       string  `json:"email" binding:"omitempty,email" example:"john.doe@example.com"`
	JoinedDate  string  `json:"joined_date" example:"2025-01-15"`
	Position    string  `json:"position" example:"Manager"`
	BaseSalary  *int    `json:"base_salary" example:"5000000"`
	LunchMoney  *int    `json:"lunch_money" example:"50000"`
}
