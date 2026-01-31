package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTherapistTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&Therapist{})
	assert.NoError(t, err)

	return db
}

func TestTherapistModel_Create(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Dr. John Smith",
		NIK:      "1234567890",
		Email:    "dr.john@test.com",
	}

	err := db.Create(&therapist).Error
	assert.NoError(t, err)
	assert.NotZero(t, therapist.ID)
}

func TestTherapistModel_Read(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Dr. Jane Doe",
		NIK:      "0987654321",
		Email:    "dr.jane@test.com",
	}
	db.Create(&therapist)

	var found Therapist
	err := db.First(&found, therapist.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "Dr. Jane Doe", found.FullName)
	assert.Equal(t, "0987654321", found.NIK)
}

func TestTherapistModel_Update(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Original Name",
		NIK:      "1111111111",
		Email:    "original@test.com",
	}
	db.Create(&therapist)

	therapist.FullName = "Updated Name"
	therapist.IsApproved = true
	err := db.Save(&therapist).Error
	assert.NoError(t, err)

	var updated Therapist
	db.First(&updated, therapist.ID)
	assert.Equal(t, "Updated Name", updated.FullName)
	assert.True(t, updated.IsApproved)
}

func TestTherapistModel_Delete(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Delete Test",
		NIK:      "2222222222",
		Email:    "delete@test.com",
	}
	db.Create(&therapist)

	err := db.Delete(&therapist).Error
	assert.NoError(t, err)

	var found Therapist
	err = db.First(&found, therapist.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestTherapistModel_ApprovalStatus(t *testing.T) {
	db := setupTherapistTestDB(t)

	// Create unapproved therapist
	therapist := Therapist{
		FullName:   "Pending Therapist",
		NIK:        "3333333333",
		Email:      "pending@test.com",
		IsApproved: false,
	}
	db.Create(&therapist)

	assert.False(t, therapist.IsApproved)

	// Approve
	therapist.IsApproved = true
	db.Save(&therapist)

	var updated Therapist
	db.First(&updated, therapist.ID)
	assert.True(t, updated.IsApproved)
}

func TestTherapistModel_ListApproved(t *testing.T) {
	db := setupTherapistTestDB(t)

	// Create approved and unapproved therapists
	for i := 0; i < 3; i++ {
		therapist := Therapist{
			FullName:   "Approved " + string(rune(i)),
			NIK:        "APPR" + string(rune(i)),
			Email:      "approved" + string(rune(i)) + "@test.com",
			IsApproved: true,
		}
		db.Create(&therapist)
	}

	for i := 0; i < 2; i++ {
		therapist := Therapist{
			FullName:   "Unapproved " + string(rune(i)),
			NIK:        "UNAPPR" + string(rune(i)),
			Email:      "unapproved" + string(rune(i)) + "@test.com",
			IsApproved: false,
		}
		db.Create(&therapist)
	}

	var approved []Therapist
	err := db.Where("is_approved = ?", true).Find(&approved).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(approved), 3)
}

func TestTherapistModel_SearchByNIK(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Search Test",
		NIK:      "SEARCH123",
		Email:    "search@test.com",
	}
	db.Create(&therapist)

	var found Therapist
	err := db.Where("NIK = ?", "SEARCH123").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, "Search Test", found.FullName)
}

func TestTherapistModel_SearchByEmail(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Email Search",
		NIK:      "EMAIL123",
		Email:    "unique.email@test.com",
	}
	db.Create(&therapist)

	var found Therapist
	err := db.Where("email = ?", "unique.email@test.com").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, "Email Search", found.FullName)
}

func TestTherapistModel_Timestamps(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Timestamp Test",
		NIK:      "TIME123",
		Email:    "timestamp@test.com",
	}
	db.Create(&therapist)

	assert.NotZero(t, therapist.CreatedAt)
	assert.NotZero(t, therapist.UpdatedAt)
}

func TestTherapistModel_CountByStatus(t *testing.T) {
	db := setupTherapistTestDB(t)

	// Create approved therapists
	for i := 0; i < 5; i++ {
		therapist := Therapist{
			FullName:   "Count Test " + string(rune(i)),
			NIK:        "COUNT" + string(rune(i)),
			Email:      "count" + string(rune(i)) + "@test.com",
			IsApproved: true,
		}
		db.Create(&therapist)
	}

	var count int64
	err := db.Model(&Therapist{}).Where("is_approved = ?", true).Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(5))
}

func TestTherapistModel_SearchByName(t *testing.T) {
	db := setupTherapistTestDB(t)

	therapist := Therapist{
		FullName: "Dr. Searchable Name",
		NIK:      "NAME123",
		Email:    "searchname@test.com",
	}
	db.Create(&therapist)

	var found []Therapist
	err := db.Where("full_name LIKE ?", "%Searchable%").Find(&found).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(found), 1)
}
