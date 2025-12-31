package model

import (
	"fmt"

	"gorm.io/gorm"
)

// Role ID constants
const (
	RoleAdmin     uint32 = 1
	RoleUser      uint32 = 2
	RoleTherapist uint32 = 3
)

type Role struct {
	gorm.Model
	ID   uint32 `gorm:"primary_key;auto_increment" json:"id"`
	Name string `gorm:"type:varchar(100);not null" json:"name"`
}

func SeedRoles(db *gorm.DB) error {
	// Define the roles you want to seed.
	roles := []Role{
		{Name: "Admin"},
		{Name: "User"},
		{Name: "Therapist"},
	}

	for _, role := range roles {
		var existingRole Role
		// Check if the role already exists.
		err := db.Where("name = ?", role.Name).First(&existingRole).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		// Create the role if not found.
		if err := db.Create(&role).Error; err != nil {
			return fmt.Errorf("failed to seed role %s: %w", role.Name, err)
		}
	}
	return nil
}
