package endpoint

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"gorm.io/gorm"
)

// setupTestDB is a helper function that sets up the test environment, database connection,
// migration, and table cleanup. It returns a *gorm.DB instance for use in tests.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

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

	// migrate patient table
	if err := db.AutoMigrate(&model.Patient{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// clean table
	db.Where("1 = 1").Delete(&model.Patient{})

	return db
}

func TestGetInitials(t *testing.T) {
	if got := getInitials("John Doe"); got != "J" {
		t.Fatalf("getInitials() = %s; want J", got)
	}
	if got := getInitials(""); got != "" {
		t.Fatalf("expected empty initials for empty name, got %s", got)
	}
}

func TestFetchPatientsDateFilter(t *testing.T) {
	db := setupTestDB(t)

	old := model.Patient{Model: gorm.Model{CreatedAt: time.Now().AddDate(0, -4, 0)}, FullName: "Old Patient", PhoneNumber: "111"}
	recent := model.Patient{Model: gorm.Model{CreatedAt: time.Now()}, FullName: "Recent Patient", PhoneNumber: "222"}

	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("create old patient: %v", err)
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("create recent patient: %v", err)
	}

	patients, _, err := fetchPatients(db, 0, 0, "", "last_3_months", "", "")
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

func TestFetchPatientsSortByFullName(t *testing.T) {
	db := setupTestDB(t)

	// Create patients in non-alphabetical order to verify sorting works correctly regardless of insertion order
	alice := model.Patient{FullName: "Alice Smith", PatientCode: "A001", PhoneNumber: "111"}
	charlie := model.Patient{FullName: "Charlie Brown", PatientCode: "C001", PhoneNumber: "333"}
	bob := model.Patient{FullName: "Bob Johnson", PatientCode: "B001", PhoneNumber: "222"}

	if err := db.Create(&alice).Error; err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := db.Create(&charlie).Error; err != nil {
		t.Fatalf("create charlie: %v", err)
	}
	if err := db.Create(&bob).Error; err != nil {
		t.Fatalf("create bob: %v", err)
	}

	// Test ascending order
	patients, _, err := fetchPatients(db, 0, 0, "", "", "full_name", "asc")
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != 3 {
		t.Fatalf("expected 3 patients, got %d", len(patients))
	}
	if patients[0].FullName != "Alice Smith" {
		t.Errorf("expected first patient to be Alice Smith, got %s", patients[0].FullName)
	}
	if patients[1].FullName != "Bob Johnson" {
		t.Errorf("expected second patient to be Bob Johnson, got %s", patients[1].FullName)
	}
	if patients[2].FullName != "Charlie Brown" {
		t.Errorf("expected third patient to be Charlie Brown, got %s", patients[2].FullName)
	}

	// Test descending order
	patients, _, err = fetchPatients(db, 0, 0, "", "", "full_name", "desc")
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != 3 {
		t.Fatalf("expected 3 patients, got %d", len(patients))
	}
	if patients[0].FullName != "Charlie Brown" {
		t.Errorf("expected first patient to be Charlie Brown, got %s", patients[0].FullName)
	}
	if patients[1].FullName != "Bob Johnson" {
		t.Errorf("expected second patient to be Bob Johnson, got %s", patients[1].FullName)
	}
	if patients[2].FullName != "Alice Smith" {
		t.Errorf("expected third patient to be Alice Smith, got %s", patients[2].FullName)
	}
}

func TestFetchPatientsSortByPatientCode(t *testing.T) {
	db := setupTestDB(t)

	// create patients with different patient codes
	patient1 := model.Patient{FullName: "Patient One", PatientCode: "P003", PhoneNumber: "111"}
	patient2 := model.Patient{FullName: "Patient Two", PatientCode: "P001", PhoneNumber: "222"}
	patient3 := model.Patient{FullName: "Patient Three", PatientCode: "P002", PhoneNumber: "333"}

	if err := db.Create(&patient1).Error; err != nil {
		t.Fatalf("create patient1: %v", err)
	}
	if err := db.Create(&patient2).Error; err != nil {
		t.Fatalf("create patient2: %v", err)
	}
	if err := db.Create(&patient3).Error; err != nil {
		t.Fatalf("create patient3: %v", err)
	}

	// Test ascending order
	patients, _, err := fetchPatients(db, 0, 0, "", "", "patient_code", "asc")
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != 3 {
		t.Fatalf("expected 3 patients, got %d", len(patients))
	}
	if patients[0].PatientCode != "P001" {
		t.Errorf("expected first patient code to be P001, got %s", patients[0].PatientCode)
	}
	if patients[1].PatientCode != "P002" {
		t.Errorf("expected second patient code to be P002, got %s", patients[1].PatientCode)
	}
	if patients[2].PatientCode != "P003" {
		t.Errorf("expected third patient code to be P003, got %s", patients[2].PatientCode)
	}

	// Test descending order
	patients, _, err = fetchPatients(db, 0, 0, "", "", "patient_code", "desc")
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != 3 {
		t.Fatalf("expected 3 patients, got %d", len(patients))
	}
	if patients[0].PatientCode != "P003" {
		t.Errorf("expected first patient code to be P003, got %s", patients[0].PatientCode)
	}
	if patients[1].PatientCode != "P002" {
		t.Errorf("expected second patient code to be P002, got %s", patients[1].PatientCode)
	}
	if patients[2].PatientCode != "P001" {
		t.Errorf("expected third patient code to be P001, got %s", patients[2].PatientCode)
	}
}

func TestFetchPatientsDefaultSort(t *testing.T) {
	db := setupTestDB(t)

	// create patients with explicit creation times
	now := time.Now()
	oldest := model.Patient{
		Model:       gorm.Model{CreatedAt: now.Add(-2 * time.Hour)},
		FullName:    "Oldest Patient",
		PatientCode: "O001",
		PhoneNumber: "111",
	}
	middle := model.Patient{
		Model:       gorm.Model{CreatedAt: now.Add(-1 * time.Hour)},
		FullName:    "Middle Patient",
		PatientCode: "M001",
		PhoneNumber: "222",
	}
	newest := model.Patient{
		Model:       gorm.Model{CreatedAt: now},
		FullName:    "Newest Patient",
		PatientCode: "N001",
		PhoneNumber: "333",
	}

	if err := db.Create(&oldest).Error; err != nil {
		t.Fatalf("create oldest: %v", err)
	}
	if err := db.Create(&middle).Error; err != nil {
		t.Fatalf("create middle: %v", err)
	}
	if err := db.Create(&newest).Error; err != nil {
		t.Fatalf("create newest: %v", err)
	}

	// Test default sort (should be created_at DESC)
	patients, _, err := fetchPatients(db, 0, 0, "", "", "", "")
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != 3 {
		t.Fatalf("expected 3 patients, got %d", len(patients))
	}
	// Default sort should be created_at DESC, so newest should come first
	if patients[0].FullName != "Newest Patient" {
		t.Errorf("expected first patient to be Newest Patient (default sort by created_at DESC), got %s", patients[0].FullName)
	}
	if patients[1].FullName != "Middle Patient" {
		t.Errorf("expected second patient to be Middle Patient, got %s", patients[1].FullName)
	}
	if patients[2].FullName != "Oldest Patient" {
		t.Errorf("expected third patient to be Oldest Patient, got %s", patients[2].FullName)
	}
}

func TestNormalizePhoneNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "single phone number",
			input:    []string{"081234567890"},
			expected: "081234567890",
		},
		{
			name:     "multiple phone numbers",
			input:    []string{"081234567890", "081234567891"},
			expected: "081234567890,081234567891",
		},
		{
			name:     "phone numbers with whitespace",
			input:    []string{" 081234567890 ", "081234567891"},
			expected: "081234567890,081234567891",
		},
		{
			name:     "phone numbers with empty strings",
			input:    []string{"081234567890", "", "081234567891"},
			expected: "081234567890,081234567891",
		},
		{
			name:     "phone numbers with duplicates",
			input:    []string{"081234567890", "081234567890", "081234567891"},
			expected: "081234567890,081234567891",
		},
		{
			name:     "phone numbers with duplicates and whitespace",
			input:    []string{" 081234567890 ", "081234567890", " 081234567891 "},
			expected: "081234567890,081234567891",
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: "",
		},
		{
			name:     "all empty strings",
			input:    []string{"", "  ", ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := normalizePhoneNumbers(tt.input)
			result := strings.Join(normalized, ",")
			if result != tt.expected {
				t.Errorf("normalizePhoneNumbers(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUpdatePatientPhoneNumbers(t *testing.T) {
	db := setupTestDB(t)

	// Create a patient with initial phone numbers
	initialPatient := model.Patient{
		FullName:    "Test Patient",
		PatientCode: "T001",
		PhoneNumber: "081234567890",
	}
	if err := db.Create(&initialPatient).Error; err != nil {
		t.Fatalf("create initial patient: %v", err)
	}

	tests := []struct {
		name           string
		phoneNumbers   []string
		expectedStored string
		description    string
	}{
		{
			name:           "update with new phone numbers",
			phoneNumbers:   []string{"089876543210", "081111111111"},
			expectedStored: "089876543210,081111111111",
			description:    "should store multiple phone numbers as comma-separated string",
		},
		{
			name:           "update with phone numbers containing whitespace",
			phoneNumbers:   []string{" 089876543210 ", " 081111111111 "},
			expectedStored: "089876543210,081111111111",
			description:    "should trim whitespace from phone numbers",
		},
		{
			name:           "update with duplicate phone numbers",
			phoneNumbers:   []string{"089876543210", "089876543210", "081111111111"},
			expectedStored: "089876543210,081111111111",
			description:    "should deduplicate phone numbers",
		},
		{
			name:           "update with empty strings mixed in",
			phoneNumbers:   []string{"089876543210", "", "081111111111", "  "},
			expectedStored: "089876543210,081111111111",
			description:    "should filter out empty strings",
		},
		{
			name:           "update with single phone number",
			phoneNumbers:   []string{"089876543210"},
			expectedStored: "089876543210",
			description:    "should handle single phone number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create UpdatePatientRequest with phone numbers
			updateReq := model.UpdatePatientRequest{
				PhoneNumbers: tt.phoneNumbers,
			}

			// Load the patient
			var patient model.Patient
			if err := db.First(&patient, initialPatient.ID).Error; err != nil {
				t.Fatalf("load patient: %v", err)
			}

			// Apply the phone number update logic (same as UpdatePatient endpoint)
			if len(updateReq.PhoneNumbers) > 0 {
				normalizedPhones := normalizePhoneNumbers(updateReq.PhoneNumbers)
				if len(normalizedPhones) > 0 {
					patient.PhoneNumber = strings.Join(normalizedPhones, ",")
				}
			}

			// Save the updated patient
			if err := db.Save(&patient).Error; err != nil {
				t.Fatalf("save patient: %v", err)
			}

			// Reload the patient to verify the stored value
			var reloadedPatient model.Patient
			if err := db.First(&reloadedPatient, initialPatient.ID).Error; err != nil {
				t.Fatalf("reload patient: %v", err)
			}

			if reloadedPatient.PhoneNumber != tt.expectedStored {
				t.Errorf("%s: expected phone_number to be %q, got %q", tt.description, tt.expectedStored, reloadedPatient.PhoneNumber)
			}
		})
	}
}
