package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SecurityLog represents a persisted security event
type SecurityLog struct {
	gorm.Model
	EventType string `json:"event_type" gorm:"column:event_type;type:varchar(64)"`
	UserID    string `json:"user_id" gorm:"column:user_id;type:varchar(64);index"`
	Email     string `json:"email" gorm:"column:email;type:varchar(191);index"`
	IP        string `json:"ip" gorm:"column:ip;type:varchar(45)"`
	// Location stores city and country in the format "City/Country" when available.
	Location  string         `json:"location" gorm:"column:location;type:varchar(255);index"`
	UserAgent string         `json:"user_agent" gorm:"column:user_agent;type:varchar(512)"`
	Message   string         `json:"message" gorm:"column:message;type:text"`
	Details   datatypes.JSON `json:"details" gorm:"column:details;type:json"`
}
