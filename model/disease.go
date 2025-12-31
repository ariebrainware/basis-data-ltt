package model

import "gorm.io/gorm"

// Disease represents a disease entity
// @Description Disease information
type Disease struct {
	gorm.Model
	ID          uint   `json:"id" example:"1"`
	Name        string `json:"name" example:"Diabetes"`
	Description string `json:"description" example:"A metabolic disease"`
}
