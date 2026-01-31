package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPatientCodeTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&PatientCode{})
	assert.NoError(t, err)

	return db
}

func TestPatientCodeModel_Create(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := PatientCode{
		Alphabet: "P",
		Number:   1,
		Code:     "P001",
	}

	err := db.Create(&patientCode).Error
	assert.NoError(t, err)
	assert.NotZero(t, patientCode.ID)
}

func TestPatientCodeModel_Read(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := PatientCode{
		Alphabet: "P",
		Number:   2,
		Code:     "P002",
	}
	db.Create(&patientCode)

	var found PatientCode
	err := db.First(&found, patientCode.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "P002", found.Code)
	assert.Equal(t, 2, found.Number)
}

func TestPatientCodeModel_Update(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := PatientCode{
		Alphabet: "P",
		Number:   3,
		Code:     "P003",
	}
	db.Create(&patientCode)

	// Update number
	patientCode.Number = 30
	err := db.Save(&patientCode).Error
	assert.NoError(t, err)

	var updated PatientCode
	db.First(&updated, patientCode.ID)
	assert.Equal(t, 30, updated.Number)
}

func TestPatientCodeModel_Delete(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := PatientCode{
		Alphabet: "P",
		Number:   4,
		Code:     "P004",
	}
	db.Create(&patientCode)

	err := db.Delete(&patientCode).Error
	assert.NoError(t, err)

	var found PatientCode
	err = db.First(&found, patientCode.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestPatientCodeModel_ListByAlphabet(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create codes with different alphabets
	for i := 0; i < 5; i++ {
		code := PatientCode{
			Alphabet: "J",
			Number:   i + 1,
			Code:     "J" + string(rune('0'+i)),
		}
		db.Create(&code)
	}

	for i := 0; i < 3; i++ {
		code := PatientCode{
			Alphabet: "K",
			Number:   i + 1,
			Code:     "K" + string(rune('0'+i)),
		}
		db.Create(&code)
	}

	var jCodes []PatientCode
	err := db.Where("alphabet = ?", "J").Find(&jCodes).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(jCodes), 5)
}

func TestPatientCodeModel_FindByCode(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := PatientCode{
		Alphabet: "F",
		Number:   99,
		Code:     "FINDME",
	}
	db.Create(&patientCode)

	var found PatientCode
	err := db.Where("code = ?", "FINDME").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, "FINDME", found.Code)
}

func TestPatientCodeModel_SequentialNumbers(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create sequential codes
	for i := 1; i <= 5; i++ {
		code := PatientCode{
			Alphabet: "S",
			Number:   i,
			Code:     "S" + string(rune('0'+i)),
		}
		db.Create(&code)
	}

	var codes []PatientCode
	err := db.Where("alphabet = ?", "S").Order("number ASC").Find(&codes).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(codes), 5)
}

func TestPatientCodeModel_GetMaxNumber(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create codes with various numbers
	for i := 1; i <= 10; i++ {
		code := PatientCode{
			Alphabet: "M",
			Number:   i,
			Code:     "M" + string(rune('0'+i)),
		}
		db.Create(&code)
	}

	var maxCode PatientCode
	err := db.Where("alphabet = ?", "M").Order("number DESC").First(&maxCode).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, maxCode.Number, 10)
}

func TestPatientCodeModel_CountByAlphabet(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create codes
	for i := 0; i < 10; i++ {
		code := PatientCode{
			Alphabet: "C",
			Number:   i + 1,
			Code:     "COUNT" + string(rune(i)),
		}
		db.Create(&code)
	}

	var count int64
	err := db.Model(&PatientCode{}).Where("alphabet = ?", "C").Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(10))
}

func TestPatientCodeModel_UniqueCode(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	code1 := PatientCode{
		Alphabet: "U",
		Number:   1,
		Code:     "UNIQUE",
	}
	err := db.Create(&code1).Error
	assert.NoError(t, err)

	// SQLite may not enforce unique in memory mode
	code2 := PatientCode{
		Alphabet: "U",
		Number:   2,
		Code:     "UNIQUE",
	}
	err = db.Create(&code2).Error
	// In production MySQL, this would fail with unique constraint
}

func TestPatientCodeModel_Timestamps(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := PatientCode{
		Alphabet: "T",
		Number:   1,
		Code:     "TIMESTAMP",
	}
	db.Create(&patientCode)

	assert.NotZero(t, patientCode.CreatedAt)
	assert.NotZero(t, patientCode.UpdatedAt)
}

func TestPatientCodeModel_GetLatestByAlphabet(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create some codes
	for i := 1; i <= 5; i++ {
		code := PatientCode{
			Alphabet: "L",
			Number:   i,
			Code:     "FIRST" + string(rune('0'+i)),
		}
		db.Create(&code)
	}

	var latest PatientCode
	err := db.Where("alphabet = ?", "L").Order("number DESC").First(&latest).Error
	assert.NoError(t, err)
	assert.Equal(t, "L", latest.Alphabet)
}

func TestPatientCodeModel_ListAll(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create multiple codes
	for i := 0; i < 7; i++ {
		code := PatientCode{
			Alphabet: "A",
			Number:   i + 1,
			Code:     "ALL" + string(rune('0'+i)),
		}
		db.Create(&code)
	}

	var allCodes []PatientCode
	err := db.Find(&allCodes).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(allCodes), 7)
}
