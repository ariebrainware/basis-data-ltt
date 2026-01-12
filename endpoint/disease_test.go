package endpoint

import (
	"os"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
)

func TestDiseaseCodenameUniqueness(t *testing.T) {
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret")

	// preserve and restore global JWT secret used by util
	prevSecret := os.Getenv("JWTSECRET")
	util.SetJWTSecret("test-secret")
	t.Cleanup(func() {
		util.SetJWTSecret(prevSecret)
	})

	// connect to in-memory DB
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	// migrate disease table
	if err := db.AutoMigrate(&model.Disease{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// clean table
	db.Where("1 = 1").Delete(&model.Disease{})

	// Create first disease with codename
	disease1 := model.Disease{
		Name:        "Diabetes Type 1",
		Codename:    "diabetes",
		Description: "Type 1 diabetes",
	}
	if err := db.Create(&disease1).Error; err != nil {
		t.Fatalf("create first disease: %v", err)
	}

	// Try to create second disease with same codename (should fail)
	disease2 := model.Disease{
		Name:        "Diabetes Type 2",
		Codename:    "diabetes",
		Description: "Type 2 diabetes",
	}
	err = db.Create(&disease2).Error
	if err == nil {
		t.Fatalf("expected error when creating disease with duplicate codename, got nil")
	}

	// Create disease with different codename (should succeed)
	disease3 := model.Disease{
		Name:        "Hypertension",
		Codename:    "hypertension",
		Description: "High blood pressure",
	}
	if err := db.Create(&disease3).Error; err != nil {
		t.Fatalf("create disease with unique codename: %v", err)
	}

	// Verify we can query by codename
	var found model.Disease
	if err := db.Where("codename = ?", "hypertension").First(&found).Error; err != nil {
		t.Fatalf("query disease by codename: %v", err)
	}
	if found.Name != "Hypertension" {
		t.Fatalf("expected disease name 'Hypertension', got '%s'", found.Name)
	}
}
