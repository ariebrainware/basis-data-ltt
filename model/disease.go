package model

import "gorm.io/gorm"

// Disease represents a disease entity
// @Description Disease information
type Disease struct {
	gorm.Model
	Name        string `json:"name" example:"Diabetes"`
	Description string `json:"description" example:"A metabolic disease"`
}
