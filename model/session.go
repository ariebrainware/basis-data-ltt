package model

import (
	"time"

	"gorm.io/gorm"
)

// Session represents a user session.
type Session struct {
	gorm.Model
	SessionToken string    `gorm:"unique;not null"`
	UserID       uint      `gorm:"not null"` // assume session is linked to a user
	ExpiresAt    time.Time `gorm:"not null"`
}
