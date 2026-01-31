package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
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

	createPatients(t, db, []model.Patient{old, recent})

	assertFetchOrder(t, db, listQuery{Limit: 0, Offset: 0, Keyword: "", GroupByDate: "last_3_months", SortBy: "", SortDir: ""}, func(p model.Patient) string { return p.FullName }, []string{"Recent Patient"})
}

// SortTestCase groups parameters for a sort order test.
type SortTestCase struct {
	SortBy       string
	Patients     []model.Patient
	Extract      func(model.Patient) string
	AscExpected  []string
	DescExpected []string
}

func TestFetchPatientsSortOrder(t *testing.T) {
	tests := []SortTestCase{
		{
			SortBy: "full_name",
			Patients: []model.Patient{
				{FullName: "Alice Smith", PatientCode: "A001", PhoneNumber: "111"},
				{FullName: "Charlie Brown", PatientCode: "C001", PhoneNumber: "333"},
				{FullName: "Bob Johnson", PatientCode: "B001", PhoneNumber: "222"},
			},
			Extract:      func(p model.Patient) string { return p.FullName },
			AscExpected:  []string{"Alice Smith", "Bob Johnson", "Charlie Brown"},
			DescExpected: []string{"Charlie Brown", "Bob Johnson", "Alice Smith"},
		},
		{
			SortBy: "patient_code",
			Patients: []model.Patient{
				{FullName: "Patient One", PatientCode: "P003", PhoneNumber: "111"},
				{FullName: "Patient Two", PatientCode: "P001", PhoneNumber: "222"},
				{FullName: "Patient Three", PatientCode: "P002", PhoneNumber: "333"},
			},
			Extract:      func(p model.Patient) string { return p.PatientCode },
			AscExpected:  []string{"P001", "P002", "P003"},
			DescExpected: []string{"P003", "P002", "P001"},
		},
	}

	for _, tc := range tests {
		t.Run("sort by "+tc.SortBy+" asc", func(t *testing.T) {
			db := setupTestDB(t)
			createPatients(t, db, tc.Patients)
			assertFetchOrder(t, db, listQuery{Limit: 0, Offset: 0, Keyword: "", GroupByDate: "", SortBy: tc.SortBy, SortDir: "asc"}, tc.Extract, tc.AscExpected)
		})

		t.Run("sort by "+tc.SortBy+" desc", func(t *testing.T) {
			db := setupTestDB(t)
			createPatients(t, db, tc.Patients)
			assertFetchOrder(t, db, listQuery{Limit: 0, Offset: 0, Keyword: "", GroupByDate: "", SortBy: tc.SortBy, SortDir: "desc"}, tc.Extract, tc.DescExpected)
		})
	}
}

func TestFetchPatientsDefaultSort(t *testing.T) {
	db := setupTestDB(t)

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

	createPatients(t, db, []model.Patient{oldest, middle, newest})

	assertFetchOrder(t, db, listQuery{Limit: 0, Offset: 0, Keyword: "", GroupByDate: "", SortBy: "", SortDir: ""}, func(p model.Patient) string { return p.FullName }, []string{"Newest Patient", "Middle Patient", "Oldest Patient"})
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

// test helpers
func setupRouterWithDB(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))
	r.PATCH("/patient/:id", UpdatePatient)
	return r
}

func createTestPatient(t *testing.T, db *gorm.DB) model.Patient {
	t.Helper()
	patient := model.Patient{
		FullName:    "Test Patient",
		PatientCode: fmt.Sprintf("T%d", time.Now().UnixNano()),
		PhoneNumber: "081234567890",
	}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatalf("create patient: %v", err)
	}
	return patient
}

func doPatchPatient(t *testing.T, r *gin.Engine, id uint, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	url := fmt.Sprintf("/patient/%d", id)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// createPatients inserts multiple patients into the DB and fails the test on error.
func createPatients(t *testing.T, db *gorm.DB, patients []model.Patient) {
	t.Helper()
	for _, p := range patients {
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("create patient: %v", err)
		}
	}
}

// assertFetchOrder calls fetchPatients with the provided query and asserts the
// ordered values extracted by `extract` match `expected`.
func assertFetchOrder(t *testing.T, db *gorm.DB, q listQuery, extract func(model.Patient) string, expected []string) {
	t.Helper()
	patients, _, err := fetchPatients(db, q)
	if err != nil {
		t.Fatalf("fetchPatients error: %v", err)
	}
	if len(patients) != len(expected) {
		t.Fatalf("expected %d patients, got %d", len(expected), len(patients))
	}
	for i := range expected {
		if extract(patients[i]) != expected[i] {
			t.Errorf("expected %q at pos %d, got %q", expected[i], i, extract(patients[i]))
		}
	}
}

func TestUpdatePatientPhoneNumbers(t *testing.T) {
	db := setupTestDB(t)
	r := setupRouterWithDB(db)

	tests := []struct {
		name           string
		phoneNumbers   []string
		expectedStored string
		description    string
	}{
		{"update with new phone numbers", []string{"089876543210", "081111111111"}, "089876543210,081111111111", "should store multiple phone numbers as comma-separated string"},
		{"update with phone numbers containing whitespace", []string{" 089876543210 ", " 081111111111 "}, "089876543210,081111111111", "should trim whitespace from phone numbers"},
		{"update with duplicate phone numbers", []string{"089876543210", "089876543210", "081111111111"}, "089876543210,081111111111", "should deduplicate phone numbers"},
		{"update with empty strings mixed in", []string{"089876543210", "", "081111111111", "  "}, "089876543210,081111111111", "should filter out empty strings"},
		{"update with single phone number", []string{"089876543210"}, "089876543210", "should handle single phone number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patient := createTestPatient(t, db)

			rr := doPatchPatient(t, r, patient.ID, map[string]interface{}{"phone_number": tt.phoneNumbers})

			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var reloadedPatient model.Patient
			if err := db.First(&reloadedPatient, patient.ID).Error; err != nil {
				t.Fatalf("reload patient: %v", err)
			}

			if reloadedPatient.PhoneNumber != tt.expectedStored {
				t.Errorf("%s: expected phone_number to be %q, got %q", tt.description, tt.expectedStored, reloadedPatient.PhoneNumber)
			}
		})
	}
}
