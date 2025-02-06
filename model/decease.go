package model

import "gorm.io/gorm"

type Decease struct {
	gorm.Model
	Name        string `json:"name"`
	Description string `json:"description"`
}
