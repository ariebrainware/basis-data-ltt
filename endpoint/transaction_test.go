package endpoint

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/stretchr/testify/assert"
)

func TestUpdateTransaction_AllowsNewPaymentStatuses(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Update",
		PatientCode: "TP007",
		Email:       "transaction-patient-update@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Update",
		NIK:      "NIK-TRX-007",
		Email:    "transaction-therapist-update@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	treatment := model.Treatment{
		TreatmentDate: "2026-04-16",
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Update issue",
		Treatment:     "Update therapy",
		Remarks:       "Update session",
		NextVisit:     "2026-04-23",
	}
	assert.NoError(t, db.Create(&treatment).Error)

	transaction := model.Transaction{
		TreatmentID:   treatment.ID,
		TherapistID:   therapist.ID,
		Amount:        180000,
		Remarks:       "Initial status",
		PaymentMethod: "cash",
		PaymentStatus: "unpaid",
	}
	assert.NoError(t, db.Create(&transaction).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPatch,
		registerPath: "/transaction/:id",
		requestPath:  "/transaction/" + strconv.FormatUint(uint64(transaction.ID), 10),
		handler:      UpdateTransaction,
		body: map[string]interface{}{
			"payment_status": "transfer",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	updated := response["data"].(map[string]interface{})
	assert.Equal(t, "transfer", updated["payment_status"])
}

func TestUpdateTransaction_RecalculatesAmountAndDeductsItemStock(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Item Update",
		PatientCode: "TP008",
		Email:       "transaction-patient-item-update@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Item Update",
		NIK:      "NIK-TRX-008",
		Email:    "transaction-therapist-item-update@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	assert.NoError(t, db.Create(&model.Pricing{TherapistID: therapist.ID, Price: 175000}).Error)

	itemOne := model.Item{Name: "Bandage", Quantity: 10, Price: 15000}
	assert.NoError(t, db.Create(&itemOne).Error)

	itemTwo := model.Item{Name: "Saline", Quantity: 8, Price: 25000}
	assert.NoError(t, db.Create(&itemTwo).Error)

	treatment := model.Treatment{
		TreatmentDate: "2026-04-18",
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Item update issue",
		Treatment:     "Item update therapy",
		Remarks:       "Item update session",
		NextVisit:     "2026-04-25",
	}
	assert.NoError(t, db.Create(&treatment).Error)

	transaction := model.Transaction{
		TreatmentID:   treatment.ID,
		TherapistID:   therapist.ID,
		Amount:        175000,
		Remarks:       "Initial amount",
		PaymentMethod: "cash",
		PaymentStatus: "unpaid",
	}
	assert.NoError(t, db.Create(&transaction).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPatch,
		registerPath: "/transaction/:id",
		requestPath:  "/transaction/" + strconv.FormatUint(uint64(transaction.ID), 10),
		handler:      UpdateTransaction,
		body: map[string]interface{}{
			"items": []map[string]interface{}{
				{"item_id": itemOne.ID, "quantity": 2},
				{"item_id": itemTwo.ID, "quantity": 3},
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	updated := response["data"].(map[string]interface{})
	assert.Equal(t, float64(280000), updated["amount"])

	var refreshedItemOne model.Item
	assert.NoError(t, db.First(&refreshedItemOne, itemOne.ID).Error)
	assert.Equal(t, 8, refreshedItemOne.Quantity)

	var refreshedItemTwo model.Item
	assert.NoError(t, db.First(&refreshedItemTwo, itemTwo.ID).Error)
	assert.Equal(t, 5, refreshedItemTwo.Quantity)
}

func TestListTransactions_ReturnsPatientName(t *testing.T) {
	r, db := setupEndpointTest(t)
	now := time.Now()

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
		TreatmentDate: now.Format("2006-01-02"),
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Headache",
		Treatment:     "Massage",
		Remarks:       "Initial session",
		NextVisit:     now.AddDate(0, 0, 7).Format("2006-01-02"),
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
	assert.Equal(t, now.Format("2006-01-02"), first["treatment_date"])

	// Verify summary is present
	summary := data["summary"].(map[string]interface{})
	assert.NotNil(t, summary)
	assert.Equal(t, float64(125000), summary["total_amount"]) // Should be the amount we created today

	paymentStatusCounts := summary["payment_status_counts"].(map[string]interface{})
	assert.Equal(t, float64(1), paymentStatusCounts["paid"]) // Created transaction is paid

	therapistPatientCounts := summary["therapist_patient_counts"].([]interface{})
	assert.Greater(t, len(therapistPatientCounts), 0)
}

func TestListTransactions_WithDateFilter(t *testing.T) {
	r, db := setupEndpointTest(t)
	now := time.Now()

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

	today := now.Format("2006-01-02")
	otherDate := now.AddDate(0, 0, -1).Format("2006-01-02")

	todayTreatment := model.Treatment{
		TreatmentDate: today,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Neck pain",
		Treatment:     "Therapy A",
		Remarks:       "Today session",
		NextVisit:     now.AddDate(0, 0, 7).Format("2006-01-02"),
	}
	assert.NoError(t, db.Create(&todayTreatment).Error)

	oldTreatment := model.Treatment{
		TreatmentDate: otherDate,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Back pain",
		Treatment:     "Therapy B",
		Remarks:       "Old session",
		NextVisit:     now.AddDate(0, 0, 8).Format("2006-01-02"),
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
		requestPath:  "/transaction?treatment_date=" + today,
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
	assert.Equal(t, float64(100000), summary["total_amount"])

	paymentStatusCounts := summary["payment_status_counts"].(map[string]interface{})
	assert.Equal(t, float64(1), paymentStatusCounts["paid"])
	assert.Equal(t, float64(0), paymentStatusCounts["unpaid"])

	therapistPatientCounts := summary["therapist_patient_counts"].([]interface{})
	assert.Len(t, therapistPatientCounts, 1)
	firstTherapist := therapistPatientCounts[0].(map[string]interface{})
	assert.Equal(t, "Therapist Beta", firstTherapist["therapist_name"])
	assert.Equal(t, float64(1), firstTherapist["patient_count"])
}

func TestListTransactions_WithoutDateFilterSummaryMatchesReturnedScope(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Epsilon",
		PatientCode: "TP006",
		Email:       "transaction-patient-epsilon@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Epsilon",
		NIK:      "NIK-TRX-006",
		Email:    "transaction-therapist-epsilon@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	treatmentOne := model.Treatment{
		TreatmentDate: "2026-04-14",
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Issue one",
		Treatment:     "Therapy one",
		Remarks:       "First",
		NextVisit:     "2026-04-20",
	}
	assert.NoError(t, db.Create(&treatmentOne).Error)

	treatmentTwo := model.Treatment{
		TreatmentDate: "2026-04-15",
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Issue two",
		Treatment:     "Therapy two",
		Remarks:       "Second",
		NextVisit:     "2026-04-21",
	}
	assert.NoError(t, db.Create(&treatmentTwo).Error)

	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   treatmentOne.ID,
		TherapistID:   therapist.ID,
		Amount:        100000,
		PaymentMethod: "cash",
		PaymentStatus: "paid",
	}).Error)
	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   treatmentTwo.ID,
		TherapistID:   therapist.ID,
		Amount:        200000,
		PaymentMethod: "cash",
		PaymentStatus: "unpaid",
	}).Error)

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
	assert.Len(t, transactions, 2)

	summary := data["summary"].(map[string]interface{})
	assert.Equal(t, float64(300000), summary["total_amount"])
}

func TestListTransactions_WithTreatmentDateAndISODate(t *testing.T) {
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
		requestPath:  "/transaction?treatment_date=2026-04-10T00:00:00.000Z",
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

func TestListTransactions_WithTreatmentDateAlias(t *testing.T) {
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
		requestPath:  "/transaction?treatment_date=" + targetDate,
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

func TestListTransactions_WithDateRange(t *testing.T) {
	r, db := setupEndpointTest(t)

	patient := model.Patient{
		FullName:    "Patient Range",
		PatientCode: "TP005",
		Email:       "transaction-patient-range@example.com",
	}
	assert.NoError(t, db.Create(&patient).Error)

	therapist := model.Therapist{
		FullName: "Therapist Range",
		NIK:      "NIK-TRX-005",
		Email:    "transaction-therapist-range@example.com",
	}
	assert.NoError(t, db.Create(&therapist).Error)

	dateOne := "2026-04-01"
	dateTwo := "2026-04-02"
	dateThree := "2026-04-04"

	treatmentOne := model.Treatment{
		TreatmentDate: dateOne,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "First issue",
		Treatment:     "Range therapy 1",
		Remarks:       "First range day",
		NextVisit:     "2026-04-08",
	}
	assert.NoError(t, db.Create(&treatmentOne).Error)

	treatmentTwo := model.Treatment{
		TreatmentDate: dateTwo,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Second issue",
		Treatment:     "Range therapy 2",
		Remarks:       "Second range day",
		NextVisit:     "2026-04-09",
	}
	assert.NoError(t, db.Create(&treatmentTwo).Error)

	treatmentThree := model.Treatment{
		TreatmentDate: dateThree,
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Outside issue",
		Treatment:     "Outside therapy",
		Remarks:       "Outside range",
		NextVisit:     "2026-04-10",
	}
	assert.NoError(t, db.Create(&treatmentThree).Error)

	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   treatmentOne.ID,
		TherapistID:   therapist.ID,
		Amount:        100000,
		PaymentMethod: "cash",
		PaymentStatus: "paid",
	}).Error)
	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   treatmentTwo.ID,
		TherapistID:   therapist.ID,
		Amount:        200000,
		PaymentMethod: "cash",
		PaymentStatus: "partial",
	}).Error)
	assert.NoError(t, db.Create(&model.Transaction{
		TreatmentID:   treatmentThree.ID,
		TherapistID:   therapist.ID,
		Amount:        300000,
		PaymentMethod: "cash",
		PaymentStatus: "unpaid",
	}).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/transaction",
		requestPath:  "/transaction?start_date=2026-04-01&end_date=2026-04-02",
		handler:      ListTransactions,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	transactions := data["transactions"].([]interface{})
	assert.Len(t, transactions, 2)

	first := transactions[0].(map[string]interface{})
	second := transactions[1].(map[string]interface{})
	assert.Equal(t, dateTwo, first["treatment_date"])
	assert.Equal(t, dateOne, second["treatment_date"])

	summary := data["summary"].(map[string]interface{})
	assert.Equal(t, float64(300000), summary["total_amount"])

	paymentStatusCounts := summary["payment_status_counts"].(map[string]interface{})
	assert.Equal(t, float64(1), paymentStatusCounts["paid"])
	assert.Equal(t, float64(1), paymentStatusCounts["partial"])
	assert.Equal(t, float64(0), paymentStatusCounts["unpaid"])
}
