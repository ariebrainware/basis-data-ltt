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
func createTreatment(db *gorm.DB, p CreateTreatmentParams) Treatment {
	treatment := Treatment{
		PatientCode:   p.PatientCode,
		TherapistID:   p.TherapistID,
		TreatmentDate: p.TreatmentDate,
		Issues:        p.Issues,
		Treatment:     p.Treatment,
		Remarks:       p.Remarks,
		NextVisit:     p.NextVisit,
	}
	if err := db.Create(&treatment).Error; err != nil {
		panic(fmt.Sprintf("failed to create treatment in test: %v", err))
	}
	return treatment
}

// Date formatting helpers
func todayStr() string {
	return time.Now().Format("2006-01-02")
}

func daysFromNowStr(days int) string {
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

func TestTreatmentModel_Create(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P001",
		TherapistID:   1,
		TreatmentDate: todayStr(),
		Issues:        "Back pain",
		Treatment:     "Massage therapy",
		Remarks:       "Initial treatment session",
		NextVisit:     daysFromNowStr(7),
	})
	assert.NotZero(t, treatment.ID)
}

func TestTreatmentModel_Read(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P002",
		TherapistID:   1,
		TreatmentDate: "2024-01-15",
		Issues:        "Neck pain",
		Treatment:     "Physical therapy",
		Remarks:       "Follow-up treatment",
		NextVisit:     "2024-01-22",
	})

	var found Treatment
	err := db.First(&found, treatment.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "P002", found.PatientCode)
	assert.Equal(t, "Follow-up treatment", found.Remarks)
}

func TestTreatmentModel_Update(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P003",
		TherapistID:   1,
		TreatmentDate: "2024-01-20",
		Issues:        "Shoulder pain",
		Treatment:     "Exercise",
		Remarks:       "Original remarks",
		NextVisit:     "2024-01-27",
	})

	treatment.Remarks = "Updated remarks"
	err := db.Save(&treatment).Error
	assert.NoError(t, err)

	var updated Treatment
	db.First(&updated, treatment.ID)
	assert.Equal(t, "Updated remarks", updated.Remarks)
}

func TestTreatmentModel_Delete(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P004",
		TherapistID:   1,
		TreatmentDate: "2024-01-25",
		Issues:        "Knee pain",
		Treatment:     "Rest",
		Remarks:       "Delete test",
		NextVisit:     "2024-02-01",
	})

	err := db.Delete(&treatment).Error
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
		createTreatment(db, CreateTreatmentParams{
			PatientCode:   patientCode,
			TherapistID:   1,
			TreatmentDate: daysFromNowStr(-i),
			Issues:        "Pain type " + string(rune('A'+i)),
			Treatment:     "Treatment " + string(rune('A'+i)),
			Remarks:       "Session " + string(rune('1'+i)),
			NextVisit:     daysFromNowStr(7 - i),
		})
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
		createTreatment(db, CreateTreatmentParams{
			PatientCode:   "P00" + string(rune('6'+i)),
			TherapistID:   therapistID,
			TreatmentDate: daysFromNowStr(-i),
			Issues:        "Issue " + string(rune('A'+i)),
			Treatment:     "Treatment " + string(rune('A'+i)),
			Remarks:       "Remarks " + string(rune('A'+i)),
			NextVisit:     daysFromNowStr(7 - i),
		})
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
		createTreatment(db, CreateTreatmentParams{
			PatientCode:   "P10" + string(rune('0'+i)),
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

	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P999",
		TherapistID:   10,
		TreatmentDate: "2024-03-01",
		Issues:        "Multiple issues",
		Treatment:     "Comprehensive treatment with all fields filled",
		Remarks:       "Patient responded well",
		NextVisit:     "2024-03-08",
	})

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, "P999", found.PatientCode)
	assert.Equal(t, uint(10), found.TherapistID)
	assert.Equal(t, "2024-03-01", found.TreatmentDate)
	assert.Equal(t, "Comprehensive treatment with all fields filled", found.Treatment)
}

func TestTreatmentModel_Timestamps(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P100",
		TherapistID:   1,
		TreatmentDate: todayStr(),
		Issues:        "Timestamp test issue",
		Treatment:     "Timestamp test treatment",
		Remarks:       "Testing timestamps",
		NextVisit:     daysFromNowStr(7),
	})

	assert.NotZero(t, treatment.CreatedAt)
	assert.NotZero(t, treatment.UpdatedAt)
}

func TestTreatmentModel_OrderByDate(t *testing.T) {
	db := setupTreatmentTestDB(t)

	// Create treatments on different dates
	dates := []string{"2024-01-01", "2024-01-15", "2024-01-10"}
	for i, date := range dates {
		createTreatment(db, CreateTreatmentParams{
			PatientCode:   "PORD" + string(rune('0'+i)),
			TherapistID:   1,
			TreatmentDate: date,
			Issues:        "Order test issue " + string(rune('A'+i)),
			Treatment:     "Order test " + string(rune('A'+i)),
			Remarks:       "Testing order",
			NextVisit:     "2024-02-01",
		})
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
		createTreatment(db, CreateTreatmentParams{
			PatientCode:   patientCode,
			TherapistID:   1,
			TreatmentDate: daysFromNowStr(-i),
			Issues:        "Count test issue",
			Treatment:     "Count test treatment",
			Remarks:       "Testing count",
			NextVisit:     daysFromNowStr(7 - i),
		})
	}

	var count int64
	err := db.Model(&Treatment{}).Where("patient_code = ?", patientCode).Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(6))
}

func TestTreatmentModel_EmptyRemarks(t *testing.T) {
	db := setupTreatmentTestDB(t)
	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P200",
		TherapistID:   1,
		TreatmentDate: todayStr(),
		Issues:        "Empty remarks test",
		Treatment:     "Treatment with no remarks",
		Remarks:       "",
		NextVisit:     daysFromNowStr(7),
	})

	var found Treatment
	db.First(&found, treatment.ID)
	assert.Equal(t, "", found.Remarks)
}

func TestTreatmentModel_LongRemarks(t *testing.T) {
	db := setupTreatmentTestDB(t)

	longRemarks := "This is a very long remark that contains detailed information about the therapy session. " +
		"It includes observations, progress notes, recommendations for future treatments, and any concerns " +
		"that need to be addressed. The remark may span multiple paragraphs and contain specific medical terminology."

	treatment := createTreatment(db, CreateTreatmentParams{
		PatientCode:   "P300",
		TherapistID:   1,
		TreatmentDate: todayStr(),
		Issues:        "Complex case",
		Treatment:     "Comprehensive therapy",
		Remarks:       longRemarks,
		NextVisit:     daysFromNowStr(7),
	})

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
