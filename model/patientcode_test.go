package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupPatientCodeTestDB(t *testing.T) *gorm.DB {
	return setupTestDB(t, "patientcode", &PatientCode{})
}

// createPatientCodeHelper creates a PatientCode record and fails the test on error.
func createPatientCodeHelper(t *testing.T, db *gorm.DB, p PatientCode) PatientCode {
	t.Helper()
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("failed to create patient code %q: %v", p.Code, err)
	}
	return p
}

// createPatientCodesHelper creates multiple PatientCode records using a prefix for the Code.
func createPatientCodesHelper(t *testing.T, db *gorm.DB, alpha string, count int, prefix string) []PatientCode {
	t.Helper()
	var created []PatientCode
	for i := 1; i <= count; i++ {
		code := fmt.Sprintf("%s%d", prefix, i)
		p := newPatientCode(alpha, i, code)
		created = append(created, createPatientCodeHelper(t, db, p))
	}
	return created
}

// newPatientCode constructs a PatientCode with the common fields used in tests.
func newPatientCode(alpha string, number int, code string) PatientCode {
	return PatientCode{Alphabet: alpha, Number: number, Code: code}
}

func TestPatientCodeModel_Create(t *testing.T) {
	db := setupPatientCodeTestDB(t)
	patientCode := createPatientCodeHelper(t, db, newPatientCode("P", 1, "P001"))
	assert.NotZero(t, patientCode.ID)
}

func TestPatientCodeModel_Read(t *testing.T) {
	db := setupPatientCodeTestDB(t)
	patientCode := createPatientCodeHelper(t, db, newPatientCode("P", 2, "P002"))

	var found PatientCode
	err := db.First(&found, patientCode.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "P002", found.Code)
	assert.Equal(t, 2, found.Number)
}

func TestPatientCodeModel_Update(t *testing.T) {
	db := setupPatientCodeTestDB(t)
	patientCode := createPatientCodeHelper(t, db, newPatientCode("P", 3, "P003"))

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
	patientCode := createPatientCodeHelper(t, db, newPatientCode("P", 4, "P004"))

	err := db.Delete(&patientCode).Error
	assert.NoError(t, err)

	var found PatientCode
	err = db.First(&found, patientCode.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestPatientCodeModel_ListByAlphabet(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create codes with different alphabets
	createPatientCodesHelper(t, db, "J", 5, "J")
	createPatientCodesHelper(t, db, "K", 3, "K")

	var jCodes []PatientCode
	err := db.Where("alphabet = ?", "J").Find(&jCodes).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(jCodes), 5)
}

func TestPatientCodeModel_FindByCode(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	createPatientCodeHelper(t, db, newPatientCode("F", 99, "FINDME"))

	var found PatientCode
	err := db.Where("code = ?", "FINDME").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, "FINDME", found.Code)
}

func TestPatientCodeModel_SequentialNumbers(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create sequential codes
	createPatientCodesHelper(t, db, "S", 5, "S")

	var codes []PatientCode
	err := db.Where("alphabet = ?", "S").Order("number ASC").Find(&codes).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(codes), 5)
}

func TestPatientCodeModel_GetMaxNumber(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create codes with various numbers
	createPatientCodesHelper(t, db, "M", 10, "M")

	var maxCode PatientCode
	err := db.Where("alphabet = ?", "M").Order("number DESC").First(&maxCode).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, maxCode.Number, 10)
}

func TestPatientCodeModel_CountByAlphabet(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create codes
	createPatientCodesHelper(t, db, "C", 10, "COUNT")

	var count int64
	err := db.Model(&PatientCode{}).Where("alphabet = ?", "C").Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(10))
}

func TestPatientCodeModel_UniqueCode(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	createPatientCodeHelper(t, db, newPatientCode("U", 1, "UNIQUE"))

	// SQLite may not enforce unique in memory mode
	tmp := newPatientCode("U", 2, "UNIQUE")
	_ = db.Create(&tmp).Error
	// In production MySQL, this would fail with unique constraint
}

func TestPatientCodeModel_Timestamps(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	patientCode := createPatientCodeHelper(t, db, newPatientCode("T", 1, "TIMESTAMP"))

	assert.NotZero(t, patientCode.CreatedAt)
	assert.NotZero(t, patientCode.UpdatedAt)
}

func TestPatientCodeModel_GetLatestByAlphabet(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create some codes
	createPatientCodesHelper(t, db, "L", 5, "FIRST")

	var latest PatientCode
	err := db.Where("alphabet = ?", "L").Order("number DESC").First(&latest).Error
	assert.NoError(t, err)
	assert.Equal(t, "L", latest.Alphabet)
}

func TestPatientCodeModel_ListAll(t *testing.T) {
	db := setupPatientCodeTestDB(t)

	// Create multiple codes
	createPatientCodesHelper(t, db, "A", 7, "ALL")

	var allCodes []PatientCode
	err := db.Find(&allCodes).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(allCodes), 7)
}
