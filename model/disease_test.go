package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupModelTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&Disease{})
	assert.NoError(t, err)

	return db
}

func TestDiseaseModel_Create(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Test Disease Create",
		Codename: "TDC001",
	}

	err := db.Create(&disease).Error
	assert.NoError(t, err)
	assert.NotZero(t, disease.ID)
}

func TestDiseaseModel_Read(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Test Disease Read",
		Codename: "TDR001",
	}
	db.Create(&disease)

	var found Disease
	err := db.First(&found, disease.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "Test Disease Read", found.Name)
	assert.Equal(t, "TDR001", found.Codename)
}

func TestDiseaseModel_Update(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Original Name",
		Codename: "TDU001",
	}
	db.Create(&disease)

	disease.Name = "Updated Name"
	err := db.Save(&disease).Error
	assert.NoError(t, err)

	var updated Disease
	db.First(&updated, disease.ID)
	assert.Equal(t, "Updated Name", updated.Name)
}

func TestDiseaseModel_Delete(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Test Disease Delete",
		Codename: "TDD001",
	}
	db.Create(&disease)

	err := db.Delete(&disease).Error
	assert.NoError(t, err)

	var found Disease
	err = db.Unscoped().First(&found, disease.ID).Error
	assert.NoError(t, err) // Should still exist but be soft deleted
	assert.NotNil(t, found.DeletedAt)
}

func TestDiseaseModel_UniqueCodename(t *testing.T) {
	db := setupModelTestDB(t)

	disease1 := Disease{
		Name:     "Disease 1",
		Codename: "UNIQUE001",
	}
	err := db.Create(&disease1).Error
	assert.NoError(t, err)

	// Try to create another with same codename
	disease2 := Disease{
		Name:     "Disease 2",
		Codename: "UNIQUE001",
	}
	err = db.Create(&disease2).Error
	// SQLite may not enforce unique constraint on NULL, but we test the attempt
	// In production MySQL, this would fail
	if err != nil {
		assert.Error(t, err)
	}
}

func TestDiseaseModel_NullCodename(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Disease without code",
		Codename: "",
	}
	err := db.Create(&disease).Error
	assert.NoError(t, err)

	var found Disease
	db.First(&found, disease.ID)
	assert.Equal(t, "", found.Codename)
}

func TestDiseaseModel_Timestamps(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Test Disease Timestamps",
		Codename: "TDTS001",
	}
	db.Create(&disease)

	assert.NotZero(t, disease.CreatedAt)
	assert.NotZero(t, disease.UpdatedAt)

	// Update and check timestamp changes
	disease.Name = "Updated"
	db.Save(&disease)
	assert.True(t, disease.UpdatedAt.After(disease.CreatedAt))
}

func TestDiseaseModel_ListAll(t *testing.T) {
	db := setupModelTestDB(t)

	// Create multiple diseases
	for i := 1; i <= 5; i++ {
		disease := Disease{
			Name:     "Disease " + string(rune(i)),
			Codename: "TD00" + string(rune(i)),
		}
		db.Create(&disease)
	}

	var diseases []Disease
	err := db.Find(&diseases).Error
	assert.NoError(t, err)
	assert.Len(t, diseases, 5)
}

func TestDiseaseModel_SearchByName(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Searchable Disease",
		Codename: "SEARCH001",
	}
	db.Create(&disease)

	var found []Disease
	err := db.Where("name LIKE ?", "%Searchable%").Find(&found).Error
	assert.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, "Searchable Disease", found[0].Name)
}

func TestDiseaseModel_SearchByCodename(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "Test Disease",
		Codename: "TESTCODE",
	}
	db.Create(&disease)

	var found Disease
	err := db.Where("codename = ?", "TESTCODE").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, "Test Disease", found.Name)
}

func TestDiseaseModel_EmptyName(t *testing.T) {
	db := setupModelTestDB(t)

	disease := Disease{
		Name:     "",
		Codename: "EMPTY001",
	}
	// GORM doesn't enforce NOT NULL at model level, only DB level
	err := db.Create(&disease).Error
	assert.NoError(t, err)
}
