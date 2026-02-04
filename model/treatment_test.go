package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTreatmentTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:testdb_treatment_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&Treatment{})
	assert.NoError(t, err)

	return db
}

// CreateTreatmentParams groups parameters for creating test treatments
type CreateTreatmentParams struct {
	PatientCode   string
	TherapistID   uint
	TreatmentDate string
	Issues        string
	Treatment     string
	Remarks       string
	NextVisit     string
}

// createTreatment creates a Treatment from params and inserts it into the DB
func createTreatment(t *testing.T, db *gorm.DB, p CreateTreatmentParams) Treatment {
	t.Helper()
	treatment := Treatment{
		PatientCode:   p.PatientCode,
		TherapistID:   p.TherapistID,
		TreatmentDate: p.TreatmentDate,
		Issues:        p.Issues,
		Treatment:     p.Treatment,
		Remarks:       p.Remarks,
		NextVisit:     p.NextVisit,
	}
	err := db.Create(&treatment).Error
	assert.NoError(t, err)
	return treatment
}

// Date formatting helpers
func todayStr() string {
	return time.Now().Format("2006-01-02")
}

func daysFromNowStr(days int) string {
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

// seedSamePatientTreatments creates `count` treatments for the same patient code.
// seedTreatments creates `count` treatments using the provided builder
// function to produce CreateTreatmentParams for each index. This reduces
// duplicated loops and excessive function arguments in specialized seeders.
func seedTreatments(t *testing.T, db *gorm.DB, count int, builder func(i int) CreateTreatmentParams) {
	for i := 0; i < count; i++ {
		createTreatment(t, db, builder(i))
	}
}

// defaultTreatmentParams returns a default CreateTreatmentParams with common test values.
// Pass the returned params to createTreatment or override fields as needed.
func defaultTreatmentParams(patientCode string, therapistID uint) CreateTreatmentParams {
	return CreateTreatmentParams{
		PatientCode:   patientCode,
		TherapistID:   therapistID,
		TreatmentDate: todayStr(),
		Issues:        "Test issue",
		Treatment:     "Test treatment",
		Remarks:       "Test remark",
		NextVisit:     daysFromNowStr(7),
	}
}

// samePatientBuilder returns a builder function for seeding treatments for a single patient.
func samePatientBuilder(patientCode string, therapistID uint) func(i int) CreateTreatmentParams {
	return func(i int) CreateTreatmentParams {
		return CreateTreatmentParams{
			PatientCode:   patientCode,
			TherapistID:   therapistID,
			TreatmentDate: daysFromNowStr(-i),
			Issues:        fmt.Sprintf("Pain type %c", 'A'+i),
			Treatment:     fmt.Sprintf("Treatment %c", 'A'+i),
			Remarks:       fmt.Sprintf("Session %d", 1+i),
			NextVisit:     daysFromNowStr(7 - i),
		}
	}
}

// multiplePatientBuilder returns a builder function for seeding treatments for different patients.
func multiplePatientBuilder(codePrefix string, therapistID uint) func(i int) CreateTreatmentParams {
	return func(i int) CreateTreatmentParams {
		return CreateTreatmentParams{
			PatientCode:   fmt.Sprintf("%s%03d", codePrefix, i),
			TherapistID:   therapistID,
			TreatmentDate: daysFromNowStr(-i),
			Issues:        fmt.Sprintf("Issue %c", 'A'+i),
			Treatment:     fmt.Sprintf("Treatment %c", 'A'+i),
			Remarks:       fmt.Sprintf("Remarks %c", 'A'+i),
			NextVisit:     daysFromNowStr(7 - i),
		}
	}
}

func TestTreatmentModel_Create(t *testing.T) {
	db := setupTreatmentTestDB(t)
	p := defaultTreatmentParams("P001", 1)
	p.Issues = "Back pain"
	p.Treatment = "Massage therapy"
	p.Remarks = "Initial treatment session"
	treatment := createTreatment(t, db, p)
	assert.NotZero(t, treatment.ID)
}

func TestTreatmentModel_Read(t *testing.T) {
	db := setupTreatmentTestDB(t)
	p := defaultTreatmentParams("P002", 1)
	p.TreatmentDate = "2024-01-15"
	p.Issues = "Neck pain"
	p.Treatment = "Physical therapy"
	p.Remarks = "Follow-up treatment"
	p.NextVisit = "2024-01-22"
	treatment := createTreatment(t, db, p)

	var found Treatment
	err := db.First(&found, treatment.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "P002", found.PatientCode)
	assert.Equal(t, "Follow-up treatment", found.Remarks)
}

func TestTreatmentModel_Update(t *testing.T) {
	db := setupTreatmentTestDB(t)
	p := defaultTreatmentParams("P003", 1)
	p.TreatmentDate = "2024-01-20"
	p.Issues = "Shoulder pain"
	p.Treatment = "Exercise"
	p.Remarks = "Original remarks"
	p.NextVisit = "2024-01-27"
	treatment := createTreatment(t, db, p)

	treatment.Remarks = "Updated remarks"
	err := db.Save(&treatment).Error
	assert.NoError(t, err)

	var updated Treatment
	db.First(&updated, treatment.ID)
	assert.Equal(t, "Updated remarks", updated.Remarks)
}

func TestTreatmentModel_Delete(t *testing.T) {
	db := setupTreatmentTestDB(t)
	p := defaultTreatmentParams("P004", 1)
	p.TreatmentDate = "2024-01-25"
	p.Issues = "Knee pain"
	p.Treatment = "Rest"
	p.Remarks = "Delete test"
	p.NextVisit = "2024-02-01"
	treatment := createTreatment(t, db, p)

	err := db.Delete(&treatment).Error
	assert.NoError(t, err)

	var found Treatment
	err = db.First(&found, treatment.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestTreatmentModel_ListByPatient(t *testing.T) {
	db := setupTreatmentTestDB(t)
	patientCode := "P005"
	seedTreatments(t, db, 3, samePatientBuilder(patientCode, 1))

	var treatments []Treatment
	err := db.Where("patient_code = ?", patientCode).Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 3)
}

func TestTreatmentModel_ListByTherapist(t *testing.T) {
	db := setupTreatmentTestDB(t)

	therapistID := uint(5)
	// Create multiple treatments for same therapist
	seedTreatments(t, db, 4, multiplePatientBuilder("P006", therapistID))

	var treatments []Treatment
	err := db.Where("therapist_id = ?", therapistID).Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 4)
}

func TestTreatmentModel_ListByDate(t *testing.T) {
	db := setupTreatmentTestDB(t)

	targetDate := "2024-02-14"

	// Create treatments on specific date
	for i := 0; i < 2; i++ {
		createTreatment(t, db, CreateTreatmentParams{
			PatientCode:   fmt.Sprintf("P10%d", i),
			TherapistID:   1,
			TreatmentDate: targetDate,
			Issues:        "Special day treatment",
			Treatment:     "Valentine's Day therapy",
			Remarks:       "Scheduled session",
			NextVisit:     "2024-02-21",
		})
	}

	var treatments []Treatment
	err := db.Where("treatment_date = ?", targetDate).Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 2)
}

func TestTreatmentModel_AllFields(t *testing.T) {
	db := setupTreatmentTestDB(t)

	p := defaultTreatmentParams("P999", 10)
	p.TreatmentDate = "2024-03-01"
	p.Issues = "Multiple issues"
	p.Treatment = "Comprehensive treatment with all fields filled"
	p.Remarks = "Patient responded well"
	p.NextVisit = "2024-03-08"
	treatment := createTreatment(t, db, p)

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, "P999", found.PatientCode)
	assert.Equal(t, uint(10), found.TherapistID)
	assert.Equal(t, "2024-03-01", found.TreatmentDate)
	assert.Equal(t, "Comprehensive treatment with all fields filled", found.Treatment)
}

func TestTreatmentModel_Timestamps(t *testing.T) {
	db := setupTreatmentTestDB(t)
	p := defaultTreatmentParams("P100", 1)
	p.Issues = "Timestamp test issue"
	p.Treatment = "Timestamp test treatment"
	p.Remarks = "Testing timestamps"
	treatment := createTreatment(t, db, p)

	assert.NotZero(t, treatment.CreatedAt)
	assert.NotZero(t, treatment.UpdatedAt)
}

func TestTreatmentModel_OrderByDate(t *testing.T) {
	db := setupTreatmentTestDB(t)

	// Create treatments on different dates
	dates := []string{"2024-01-01", "2024-01-15", "2024-01-10"}
	seedTreatments(t, db, len(dates), func(i int) CreateTreatmentParams {
		return CreateTreatmentParams{
			PatientCode:   "PORD" + string(rune('0'+i)),
			TherapistID:   1,
			TreatmentDate: dates[i],
			Issues:        "Order test issue " + string(rune('A'+i)),
			Treatment:     "Order test " + string(rune('A'+i)),
			Remarks:       "Testing order",
			NextVisit:     "2024-02-01",
		}
	})

	var treatments []Treatment
	err := db.Order("treatment_date DESC").Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 3)
}

func TestTreatmentModel_CountByPatient(t *testing.T) {
	db := setupTreatmentTestDB(t)

	patientCode := "PCOUNT"

	// Create multiple treatments
	seedTreatments(t, db, 6, samePatientBuilder(patientCode, 1))

	var count int64
	err := db.Model(&Treatment{}).Where("patient_code = ?", patientCode).Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(6))
}

func TestTreatmentModel_EmptyRemarks(t *testing.T) {
	db := setupTreatmentTestDB(t)
	p := defaultTreatmentParams("P200", 1)
	p.Issues = "Empty remarks test"
	p.Treatment = "Treatment with no remarks"
	p.Remarks = ""
	treatment := createTreatment(t, db, p)

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, "", found.Remarks)
}

func TestTreatmentModel_LongRemarks(t *testing.T) {
	db := setupTreatmentTestDB(t)

	longRemarks := "This is a very long remark that contains detailed information about the therapy session. " +
		"It includes observations, progress notes, recommendations for future treatments, and any concerns " +
		"that need to be addressed. The remark may span multiple paragraphs and contain specific medical terminology."

	p := defaultTreatmentParams("P300", 1)
	p.Issues = "Complex case"
	p.Treatment = "Comprehensive therapy"
	p.Remarks = longRemarks
	treatment := createTreatment(t, db, p)

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, longRemarks, found.Remarks)
}

func TestListTreatementResponse_Structure(t *testing.T) {
	// Test the response struct which embeds Treatment
	response := ListTreatementResponse{
		Treatment: Treatment{
			Model:         gorm.Model{ID: 1},
			PatientCode:   "P001",
			TherapistID:   1,
			TreatmentDate: "2024-01-15",
			Issues:        "Back pain",
			Treatment:     "Massage therapy",
			Remarks:       "Patient improving",
			NextVisit:     "2024-01-22",
		},
		PatientName:   "John Doe",
		TherapistName: "Dr. Smith",
		Age:           30,
	}

	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "P001", response.PatientCode)
	assert.Equal(t, "John Doe", response.PatientName)
	assert.Equal(t, uint(1), response.TherapistID)
	assert.Equal(t, "Dr. Smith", response.TherapistName)
	assert.Equal(t, 30, response.Age)
	assert.Equal(t, "Patient improving", response.Treatment.Remarks)
	assert.Equal(t, "2024-01-22", response.Treatment.NextVisit)
}

func TestTreatementRequest_Structure(t *testing.T) {
	// Test the request struct
	request := TreatementRequest{
		TreatmentDate: "2024-01-15",
		PatientCode:   "P001",
		TherapistID:   1,
		Issues:        "Shoulder pain",
		Treatment:     []string{"Physical therapy", "Exercise"},
		Remarks:       "Follow-up needed",
		NextVisit:     "2024-01-22",
	}

	assert.Equal(t, "2024-01-15", request.TreatmentDate)
	assert.Equal(t, "P001", request.PatientCode)
	assert.Equal(t, uint(1), request.TherapistID)
	assert.Equal(t, 2, len(request.Treatment))
	assert.Equal(t, "Shoulder pain", request.Issues)
	// Use these fields so static analysis doesn't mark them as unused writes
	assert.Equal(t, "Follow-up needed", request.Remarks)
	assert.Equal(t, "2024-01-22", request.NextVisit)
}
