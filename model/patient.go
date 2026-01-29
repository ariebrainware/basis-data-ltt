package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// StringArray is a []string that implements sql.Scanner and driver.Valuer
// so it can be stored as JSON in the database.
type StringArray []string

// Value implements driver.Valuer - called when writing to the DB.
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(a))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner - called when reading from the DB.
func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = StringArray{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported Scan source for StringArray: %T", src)
	}
	if len(data) == 0 {
		*a = StringArray{}
		return nil
	}
	var s []string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*a = StringArray(s)
	return nil
}

// Patient represents a patient entity
// @Description Patient information
type Patient struct {
	gorm.Model
	FullName       string      `json:"full_name" gorm:"column:full_name" example:"John Doe"`
	Password       string      `json:"password" gorm:"column:password" example:"hashed_password"`
	Gender         string      `json:"gender" gorm:"column:gender" example:"Male"`
	Age            int         `json:"age" gorm:"column:age" example:"30"`
	Job            string      `json:"job" gorm:"column:job" example:"Engineer"`
	Address        string      `json:"address" gorm:"column:address" example:"123 Main St"`
	Email          string      `json:"email" gorm:"column:email" example:"john@example.com"`
	PhoneNumbers   StringArray `json:"phone_numbers" gorm:"column:phone_number;type:json" example:"[\"081234567890\",\"089876543210\"]"`
	HealthHistory  string      `json:"health_history" gorm:"column:health_history" example:"Diabetes,Hypertension"`
	SurgeryHistory string      `json:"surgery_history" gorm:"column:surgery_history" example:"Appendectomy 2020"`
	PatientCode    string      `json:"patient_code" gorm:"column:patient_code" example:"J001"`
}
