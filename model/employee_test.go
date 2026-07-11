package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupEmployeeTestDB(t *testing.T) *gorm.DB {
	return setupTestDB(t, "employee", &Employee{})
}

func TestEmployeeModel_CreateReadUpdateDelete(t *testing.T) {
	db := setupEmployeeTestDB(t)

	employee := Employee{
		NIK:         "1234567890123456",
		FullName:    "John Doe",
		Gender:      "Male",
		Address:     "123 Main St",
		Religion:    "Islam",
		PhoneNumber: "081234567890",
		Email:       "john.doe@example.com",
		JoinedDate:  "2025-01-15",
		Position:    "Staff",
		BaseSalary:  5000000,
		LunchMoney:  50000,
	}

	// Create
	assert.NoError(t, db.Create(&employee).Error)
	assert.NotZero(t, employee.ID)

	// Read
	var found Employee
	assert.NoError(t, db.First(&found, employee.ID).Error)
	assert.Equal(t, "1234567890123456", found.NIK)
	assert.Equal(t, "John Doe", found.FullName)
	assert.Equal(t, "Staff", found.Position)
	assert.Equal(t, 5000000, found.BaseSalary)

	// Update
	assert.NoError(t, db.Model(&found).Update("fullname", "John Doe Updated").Error)
	assert.NoError(t, db.First(&found, employee.ID).Error)
	assert.Equal(t, "John Doe Updated", found.FullName)

	// Delete
	assert.NoError(t, db.Delete(&found).Error)

	// Verify Soft Delete
	var deleted Employee
	assert.Error(t, db.First(&deleted, employee.ID).Error)
}

func TestEmployeeModel_UniqueNIK(t *testing.T) {
	db := setupEmployeeTestDB(t)

	e1 := Employee{
		NIK:         "1111222233334444",
		FullName:    "Alice Smith",
		Gender:      "Female",
		Address:     "456 Oak Ave",
		Religion:    "Christianity",
		PhoneNumber: "08111111111",
		Email:       "alice@example.com",
		JoinedDate:  "2026-02-20",
		Position:    "Manager",
		BaseSalary:  6000000,
		LunchMoney:  60000,
	}
	assert.NoError(t, db.Create(&e1).Error)

	e2 := Employee{
		NIK:         "1111222233334444", // Same NIK
		FullName:    "Bob Jones",
		Gender:      "Male",
		Address:     "789 Pine Rd",
		Religion:    "Buddhism",
		PhoneNumber: "08222222222",
		Email:       "bob@example.com",
		JoinedDate:  "2026-03-10",
		Position:    "Staff",
		BaseSalary:  4000000,
		LunchMoney:  45000,
	}
	// GORM / SQLite should enforce uniqueness constraint and return error
	assert.Error(t, db.Create(&e2).Error)
}
