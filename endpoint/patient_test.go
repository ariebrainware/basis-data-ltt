package endpoint

import (
	"os"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"gorm.io/gorm"
)

func TestGetInitials(t *testing.T) {
	if got := getInitials("John Doe"); got != "J" {
		t.Fatalf("getInitials() = %s; want J", got)
	}
	if got := getInitials(""); got != "" {
		t.Fatalf("expected empty initials for empty name, got %s", got)
	}
}

func TestFetchPatientsDateFilter(t *testing.T) {
	os.Setenv("APPENV", "test")
	os.Setenv("JWTSECRET", "test-secret")
	util.SetJWTSecret("test-secret")

	// connect to in-memory DB
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	// migrate patient table
	if err := db.AutoMigrate(&model.Patient{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// clean table
	db.Where("1 = 1").Delete(&model.Patient{})

	old := model.Patient{Model: gorm.Model{CreatedAt: time.Now().AddDate(0, -4, 0)}, FullName: "Old Patient", PhoneNumber: "111"}
	recent := model.Patient{Model: gorm.Model{CreatedAt: time.Now()}, FullName: "Recent Patient", PhoneNumber: "222"}

	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("create old patient: %v", err)
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("create recent patient: %v", err)
	}

	patients, _, err := fetchPatients(db, 0, 0, 0, "", "last_3_months")
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != 1 {
		t.Fatalf("expected 1 patient in last_3_months, got %d", len(patients))
	}
	if patients[0].FullName != "Recent Patient" {
		t.Fatalf("unexpected patient returned: %s", patients[0].FullName)
	}
}
