package endpoint

import (
	"net/http"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/stretchr/testify/assert"
)

func TestListTransactions_ReturnsPatientName(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Alpha",
		PatientCode: "TP001",
		Email:       "transaction-patient@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Alpha",
		NIK:      "NIK-TRX-001",
		Email:    "transaction-therapist@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	treatment := model.Treatment{
		TreatmentDate: time.Now().Format("2006-01-02"),
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Headache",
		Treatment:     "Massage",
		Remarks:       "Initial session",
		NextVisit:     time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
	}
	assert.NoError(t, db.Create(&treatment).Error)

	transaction := model.Transaction{
		TreatmentID:   treatment.ID,
		TherapistID:   therapist.ID,
		Amount:        125000,
		Remarks:       "Paid in cash",
		PaymentMethod: "cash",
		PaymentStatus: "paid",
	}
	assert.NoError(t, db.Create(&transaction).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/transaction",
		requestPath:  "/transaction",
		handler:      ListTransactions,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	transactions := data["transactions"].([]interface{})
	assert.Len(t, transactions, 1)

	first := transactions[0].(map[string]interface{})
	assert.Equal(t, "Patient Alpha", first["patient_name"])
	assert.Equal(t, time.Now().Format("2006-01-02"), first["treatment_date"])

	// Verify summary is present
	summary := data["summary"].(map[string]interface{})
	assert.NotNil(t, summary)
	assert.Equal(t, float64(125000), summary["total_amount_today"]) // Should be the amount we created today

	paymentStatusCounts := summary["payment_status_counts"].(map[string]interface{})
	assert.Equal(t, float64(1), paymentStatusCounts["paid"]) // Created transaction is paid

	therapistPatientCounts := summary["therapist_patient_counts"].([]interface{})
	assert.Greater(t, len(therapistPatientCounts), 0)
}

func TestListTransactions_WithDateFilter(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Beta",
		PatientCode: "TP002",
		Email:       "transaction-patient-beta@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Beta",
		NIK:      "NIK-TRX-002",
		Email:    "transaction-therapist-beta@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	today := time.Now().Format("2006-01-02")
	otherDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	todayTreatment := model.Treatment{
		TreatmentDate: today,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Neck pain",
		Treatment:     "Therapy A",
		Remarks:       "Today session",
		NextVisit:     time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
	}
	assert.NoError(t, db.Create(&todayTreatment).Error)

	oldTreatment := model.Treatment{
		TreatmentDate: otherDate,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Back pain",
		Treatment:     "Therapy B",
		Remarks:       "Old session",
		NextVisit:     time.Now().AddDate(0, 0, 8).Format("2006-01-02"),
	}
	assert.NoError(t, db.Create(&oldTreatment).Error)

	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   todayTreatment.ID,
		TherapistID:   therapist.ID,
		Amount:        100000,
		PaymentMethod: "cash",
		PaymentStatus: "paid",
	}).Error)

	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   oldTreatment.ID,
		TherapistID:   therapist.ID,
		Amount:        200000,
		PaymentMethod: "cash",
		PaymentStatus: "unpaid",
	}).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/transaction",
		requestPath:  "/transaction?date=" + today,
		handler:      ListTransactions,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	transactions := data["transactions"].([]interface{})
	assert.Len(t, transactions, 1)

	first := transactions[0].(map[string]interface{})
	assert.Equal(t, today, first["treatment_date"])
	assert.Equal(t, float64(100000), first["amount"])

	summary := data["summary"].(map[string]interface{})
	assert.Equal(t, float64(100000), summary["total_amount_today"])

	paymentStatusCounts := summary["payment_status_counts"].(map[string]interface{})
	assert.Equal(t, float64(1), paymentStatusCounts["paid"])
	assert.Equal(t, float64(0), paymentStatusCounts["unpaid"])

	therapistPatientCounts := summary["therapist_patient_counts"].([]interface{})
	assert.Len(t, therapistPatientCounts, 1)
	firstTherapist := therapistPatientCounts[0].(map[string]interface{})
	assert.Equal(t, "Therapist Beta", firstTherapist["therapist_name"])
	assert.Equal(t, float64(1), firstTherapist["patient_count"])
}

func TestListTransactions_WithLimiterAliasAndISODate(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Gamma",
		PatientCode: "TP003",
		Email:       "transaction-patient-gamma@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Gamma",
		NIK:      "NIK-TRX-003",
		Email:    "transaction-therapist-gamma@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	selectedDate := "2026-04-10"
	otherDate := "2026-04-09"

	selectedTreatment := model.Treatment{
		TreatmentDate: selectedDate,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Shoulder pain",
		Treatment:     "Therapy C",
		Remarks:       "Selected date",
		NextVisit:     "2026-04-17",
	}
	assert.NoError(t, db.Create(&selectedTreatment).Error)

	otherTreatment := model.Treatment{
		TreatmentDate: otherDate,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Knee pain",
		Treatment:     "Therapy D",
		Remarks:       "Other date",
		NextVisit:     "2026-04-16",
	}
	assert.NoError(t, db.Create(&otherTreatment).Error)

	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   selectedTreatment.ID,
		TherapistID:   therapist.ID,
		Amount:        150000,
		PaymentMethod: "cash",
		PaymentStatus: "paid",
	}).Error)

	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   otherTreatment.ID,
		TherapistID:   therapist.ID,
		Amount:        250000,
		PaymentMethod: "cash",
		PaymentStatus: "unpaid",
	}).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/transaction",
		requestPath:  "/transaction?limiter=2026-04-10T00:00:00.000Z",
		handler:      ListTransactions,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	transactions := data["transactions"].([]interface{})
	assert.Len(t, transactions, 1)

	first := transactions[0].(map[string]interface{})
	assert.Equal(t, selectedDate, first["treatment_date"])
	assert.Equal(t, float64(150000), first["amount"])
}

func TestListTransactions_WithGroupByDateAlias(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Delta",
		PatientCode: "TP004",
		Email:       "transaction-patient-delta@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Delta",
		NIK:      "NIK-TRX-004",
		Email:    "transaction-therapist-delta@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	targetDate := "2026-04-13"
	nonTargetDate := "2026-04-04"

	t1 := model.Treatment{
		TreatmentDate: targetDate,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Target issue",
		Treatment:     "Target therapy",
		Remarks:       "Target",
		NextVisit:     "2026-04-20",
	}
	assert.NoError(t, db.Create(&t1).Error)

	t2 := model.Treatment{
		TreatmentDate: nonTargetDate,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Other issue",
		Treatment:     "Other therapy",
		Remarks:       "Other",
		NextVisit:     "2026-04-21",
	}
	assert.NoError(t, db.Create(&t2).Error)

	assert.NoError(t, db.Create(&model.Transaction{TreatmentID: t1.ID, TherapistID: therapist.ID, Amount: 110000, PaymentMethod: "cash", PaymentStatus: "paid"}).Error)
	assert.NoError(t, db.Create(&model.Transaction{TreatmentID: t2.ID, TherapistID: therapist.ID, Amount: 210000, PaymentMethod: "cash", PaymentStatus: "paid"}).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/transaction",
		requestPath:  "/transaction?group_by_date=" + targetDate,
		handler:      ListTransactions,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	transactions := data["transactions"].([]interface{})
	assert.Len(t, transactions, 1)

	first := transactions[0].(map[string]interface{})
	assert.Equal(t, targetDate, first["treatment_date"])
	assert.Equal(t, float64(110000), first["amount"])
}
