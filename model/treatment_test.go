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

// TreatmentSpec describes the fields for creating a test Treatment.
type TreatmentSpec struct {
	PatientCode   string
	TherapistID   uint
	TreatmentDate string
	Issues        string
	Treatment     string
	Remarks       string
	NextVisit     string
}

// newTreatment returns a Treatment populated from a TreatmentSpec.
func newTreatment(spec TreatmentSpec) Treatment {
	return Treatment{
		PatientCode:   spec.PatientCode,
		TherapistID:   spec.TherapistID,
		TreatmentDate: spec.TreatmentDate,
		Issues:        spec.Issues,
		Treatment:     spec.Treatment,
		Remarks:       spec.Remarks,
		NextVisit:     spec.NextVisit,
	}
}

// createAndInsertTreatment creates a Treatment from spec, inserts it into the DB, and returns it.
func createAndInsertTreatment(db *gorm.DB, spec TreatmentSpec) (Treatment, error) {
	t := newTreatment(spec)
	err := db.Create(&t).Error
	return t, err
}

// SpecOption configures a TreatmentSpec
type SpecOption func(*TreatmentSpec)

// mkSpec builds a TreatmentSpec using functional options to keep
// the callsite argument count small.
func mkSpec(patientCode string, therapistID uint, opts ...SpecOption) TreatmentSpec {
	spec := TreatmentSpec{PatientCode: patientCode, TherapistID: therapistID}
	for _, o := range opts {
		o(&spec)
	}
	return spec
}

// Option helpers
func WithTreatmentDate(d string) SpecOption { return func(s *TreatmentSpec) { s.TreatmentDate = d } }
func WithIssues(i string) SpecOption        { return func(s *TreatmentSpec) { s.Issues = i } }
func WithTreatment(t string) SpecOption     { return func(s *TreatmentSpec) { s.Treatment = t } }
func WithRemarks(r string) SpecOption       { return func(s *TreatmentSpec) { s.Remarks = r } }
func WithNextVisit(n string) SpecOption     { return func(s *TreatmentSpec) { s.NextVisit = n } }

// Date formatting helpers to reduce duplication
func todayStr() string {
	return time.Now().Format("2006-01-02")
}

func daysFromNowStr(days int) string {
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

func TestTreatmentModel_Create(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment, err := createAndInsertTreatment(db, mkSpec("P001", uint(1),
		WithTreatmentDate(todayStr()),
		WithIssues("Back pain"),
		WithTreatment("Massage therapy"),
		WithRemarks("Initial treatment session"),
		WithNextVisit(daysFromNowStr(7)),
	))
	assert.NoError(t, err)
	assert.NotZero(t, treatment.ID)
}

func TestTreatmentModel_Read(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment, err := createAndInsertTreatment(db, mkSpec("P002", uint(1),
		WithTreatmentDate("2024-01-15"),
		WithIssues("Neck pain"),
		WithTreatment("Physical therapy"),
		WithRemarks("Follow-up treatment"),
		WithNextVisit("2024-01-22"),
	))
	assert.NoError(t, err)

	var found Treatment
	err = db.First(&found, treatment.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "P002", found.PatientCode)
	assert.Equal(t, "Follow-up treatment", found.Remarks)
}

func TestTreatmentModel_Update(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment, err := createAndInsertTreatment(db, mkSpec("P003", uint(1),
		WithTreatmentDate("2024-01-20"),
		WithIssues("Shoulder pain"),
		WithTreatment("Exercise"),
		WithRemarks("Original remarks"),
		WithNextVisit("2024-01-27"),
	))
	assert.NoError(t, err)

	treatment.Remarks = "Updated remarks"
	err = db.Save(&treatment).Error
	assert.NoError(t, err)

	var updated Treatment
	db.First(&updated, treatment.ID)
	assert.Equal(t, "Updated remarks", updated.Remarks)
}

func TestTreatmentModel_Delete(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment, err := createAndInsertTreatment(db, mkSpec("P004", uint(1),
		WithTreatmentDate("2024-01-25"),
		WithIssues("Knee pain"),
		WithTreatment("Rest"),
		WithRemarks("Delete test"),
		WithNextVisit("2024-02-01"),
	))
	assert.NoError(t, err)

	err = db.Delete(&treatment).Error
	assert.NoError(t, err)

	var found Treatment
	err = db.First(&found, treatment.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestTreatmentModel_ListByPatient(t *testing.T) {
	db := setupTreatmentTestDB(t)

	patientCode := "P005"

	// Create multiple treatments for same patient
	for i := 0; i < 3; i++ {
		_, err := createAndInsertTreatment(db, mkSpec(patientCode, uint(1),
			WithTreatmentDate(daysFromNowStr(-i)),
			WithIssues("Pain type "+string(rune('A'+i))),
			WithTreatment("Treatment "+string(rune('A'+i))),
			WithRemarks("Session "+string(rune('1'+i))),
			WithNextVisit(daysFromNowStr(7-i)),
		))
		assert.NoError(t, err)
	}

	var treatments []Treatment
	err := db.Where("patient_code = ?", patientCode).Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 3)
}

func TestTreatmentModel_ListByTherapist(t *testing.T) {
	db := setupTreatmentTestDB(t)

	therapistID := uint(5)

	// Create multiple treatments for same therapist
	for i := 0; i < 4; i++ {
		_, err := createAndInsertTreatment(db, mkSpec("P00"+string(rune('6'+i)), therapistID,
			WithTreatmentDate(daysFromNowStr(-i)),
			WithIssues("Issue "+string(rune('A'+i))),
			WithTreatment("Treatment "+string(rune('A'+i))),
			WithRemarks("Remarks "+string(rune('A'+i))),
			WithNextVisit(daysFromNowStr(7-i)),
		))
		assert.NoError(t, err)
	}

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
		_, err := createAndInsertTreatment(db, mkSpec("P10"+string(rune('0'+i)), uint(1),
			WithTreatmentDate(targetDate),
			WithIssues("Special day treatment"),
			WithTreatment("Valentine's Day therapy"),
			WithRemarks("Scheduled session"),
			WithNextVisit("2024-02-21"),
		))
		assert.NoError(t, err)
	}

	var treatments []Treatment
	err := db.Where("treatment_date = ?", targetDate).Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 2)
}

func TestTreatmentModel_AllFields(t *testing.T) {
	db := setupTreatmentTestDB(t)

	treatment, err := createAndInsertTreatment(db, mkSpec("P999", uint(10),
		WithTreatmentDate("2024-03-01"),
		WithIssues("Multiple issues"),
		WithTreatment("Comprehensive treatment with all fields filled"),
		WithRemarks("Patient responded well"),
		WithNextVisit("2024-03-08"),
	))
	assert.NoError(t, err)

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, "P999", found.PatientCode)
	assert.Equal(t, uint(10), found.TherapistID)
	assert.Equal(t, "2024-03-01", found.TreatmentDate)
	assert.Equal(t, "Comprehensive treatment with all fields filled", found.Treatment)
}

func TestTreatmentModel_Timestamps(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment, err := createAndInsertTreatment(db, mkSpec("P100", uint(1),
		WithTreatmentDate(todayStr()),
		WithIssues("Timestamp test issue"),
		WithTreatment("Timestamp test treatment"),
		WithRemarks("Testing timestamps"),
		WithNextVisit(daysFromNowStr(7)),
	))
	assert.NoError(t, err)

	assert.NotZero(t, treatment.CreatedAt)
	assert.NotZero(t, treatment.UpdatedAt)
}

func TestTreatmentModel_OrderByDate(t *testing.T) {
	db := setupTreatmentTestDB(t)

	// Create treatments on different dates
	dates := []string{"2024-01-01", "2024-01-15", "2024-01-10"}
	for i, date := range dates {
		_, err := createAndInsertTreatment(db, mkSpec("PORD"+string(rune('0'+i)), uint(1),
			WithTreatmentDate(date),
			WithIssues("Order test issue "+string(rune('A'+i))),
			WithTreatment("Order test "+string(rune('A'+i))),
			WithRemarks("Testing order"),
			WithNextVisit("2024-02-01"),
		))
		assert.NoError(t, err)
	}

	var treatments []Treatment
	err := db.Order("treatment_date DESC").Find(&treatments).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(treatments), 3)
}

func TestTreatmentModel_CountByPatient(t *testing.T) {
	db := setupTreatmentTestDB(t)

	patientCode := "PCOUNT"

	// Create multiple treatments
	for i := 0; i < 6; i++ {
		_, err := createAndInsertTreatment(db, mkSpec(patientCode, uint(1),
			WithTreatmentDate(daysFromNowStr(-i)),
			WithIssues("Count test issue"),
			WithTreatment("Count test treatment"),
			WithRemarks("Testing count"),
			WithNextVisit(daysFromNowStr(7-i)),
		))
		assert.NoError(t, err)
	}

	var count int64
	err := db.Model(&Treatment{}).Where("patient_code = ?", patientCode).Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(6))
}

func TestTreatmentModel_EmptyRemarks(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment, err := createAndInsertTreatment(db, mkSpec("P200", uint(1),
		WithTreatmentDate(todayStr()),
		WithIssues("Empty remarks test"),
		WithTreatment("Treatment with no remarks"),
		WithRemarks(""),
		WithNextVisit(daysFromNowStr(7)),
	))
	assert.NoError(t, err)

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, "", found.Remarks)
}

func TestTreatmentModel_LongRemarks(t *testing.T) {
	db := setupTreatmentTestDB(t)

	longRemarks := "This is a very long remark that contains detailed information about the therapy session. " +
		"It includes observations, progress notes, recommendations for future treatments, and any concerns " +
		"that need to be addressed. The remark may span multiple paragraphs and contain specific medical terminology."

	treatment, err := createAndInsertTreatment(db, mkSpec("P300", uint(1),
		WithTreatmentDate(todayStr()),
		WithIssues("Complex case"),
		WithTreatment("Comprehensive therapy"),
		WithRemarks(longRemarks),
		WithNextVisit(daysFromNowStr(7)),
	))
	assert.NoError(t, err)

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
