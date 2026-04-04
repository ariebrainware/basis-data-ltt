package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupPricingTestDB(t *testing.T) *gorm.DB {
	return setupTestDB(t, "pricing", &Pricing{})
}

func TestPricingModel_CreateReadUpdateDelete(t *testing.T) {
	db := setupPricingTestDB(t)

	pricing := Pricing{
		TherapistID: 2,
		Price:       250000,
	}

	assert.NoError(t, db.Create(&pricing).Error)
	assert.NotZero(t, pricing.ID)

	var found Pricing
	assert.NoError(t, db.First(&found, pricing.ID).Error)
	assert.Equal(t, int64(250000), found.Price)

	assert.NoError(t, db.Model(&found).Update("price", int64(300000)).Error)
	assert.NoError(t, db.First(&found, pricing.ID).Error)
	assert.Equal(t, int64(300000), found.Price)

	assert.NoError(t, db.Delete(&found).Error)

	var deleted Pricing
	assert.Error(t, db.First(&deleted, pricing.ID).Error)
}
