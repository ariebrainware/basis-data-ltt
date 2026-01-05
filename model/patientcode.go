package model

import "gorm.io/gorm"

type PatientCode struct {
	gorm.Model
	Alphabet string `json:"alphabet" gorm:"size:1"`
	Number   int    `json:"number"`
	Code     string `json:"code" gorm:"uniqueIndex;size:191"`
}
