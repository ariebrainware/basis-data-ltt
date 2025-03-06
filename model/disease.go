package model

import "gorm.io/gorm"

type Disease struct {
	gorm.Model
	Name        string `json:"name"`
	Description string `json:"description"`
}
