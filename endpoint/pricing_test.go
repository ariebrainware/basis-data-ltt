package endpoint

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupPricingEndpointTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	r, db := setupEndpointTest(t)
	return r, db
}

func createPricingPrerequisites(t *testing.T, db *gorm.DB) (model.Treatment, model.Therapist) {
	t.Helper()

	therapist := model.Therapist{
		FullName: "Pricing Therapist",
		NIK:      fmt.Sprintf("NIK%d", time.Now().UnixNano()),
		Email:    fmt.Sprintf("pricing.therapist.%d@test.com", time.Now().UnixNano()),
	}
	if err := db.Create(&therapist).Error; err != nil {
		t.Fatalf("create therapist: %v", err)
	}

	patient := model.Patient{
		FullName:    "Pricing Patient",
		PatientCode: fmt.Sprintf("PC%d", time.Now().UnixNano()),
		Email:       fmt.Sprintf("pricing.patient.%d@test.com", time.Now().UnixNano()),
	}
	if err := db.Create(&patient).Error; err != nil {
		t.Fatalf("create patient: %v", err)
	}

	treatment := model.Treatment{
		TreatmentDate: time.Now().Format("2006-01-02"),
		PatientCode:   patient.PatientCode,
		TherapistID:   therapist.ID,
		Issues:        "Pricing issues",
		Treatment:     "Pricing treatment",
		Remarks:       "Pricing remarks",
		NextVisit:     time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
	}
	if err := db.Create(&treatment).Error; err != nil {
		t.Fatalf("create treatment: %v", err)
	}

	return treatment, therapist
}

func createPricingRecordForTest(t *testing.T, db *gorm.DB, price int64) model.Pricing {
	t.Helper()
	treatment, therapist := createPricingPrerequisites(t, db)
	pricing := model.Pricing{TreatmentID: treatment.ID, TherapistID: therapist.ID, Price: price}
	if err := db.Create(&pricing).Error; err != nil {
		t.Fatalf("create pricing: %v", err)
	}
	return pricing
}

func doPricingIDRequest(
	r *gin.Engine,
	method string,
	handler gin.HandlerFunc,
	id uint,
	body interface{},
) (int, map[string]interface{}, error) {
	path := fmt.Sprintf("/pricing/%d", id)
	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       method,
		registerPath: "/pricing/:id",
		requestPath:  path,
		handler:      handler,
		body:         body,
	})
	return w.Code, response, err
}

func TestCreatePricing_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	treatment, therapist := createPricingPrerequisites(t, db)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/pricing",
		requestPath:  "/pricing",
		handler:      CreatePricing,
		body: map[string]interface{}{
			"treatment_id": treatment.ID,
			"therapist_id": therapist.ID,
			"price":        int64(275000),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	var pricing model.Pricing
	assert.NoError(t, db.Where("treatment_id = ?", treatment.ID).First(&pricing).Error)
	assert.Equal(t, int64(275000), pricing.Price)
}

func TestCreatePricing_DuplicateTreatment(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	treatment, therapist := createPricingPrerequisites(t, db)

	first := model.Pricing{TreatmentID: treatment.ID, TherapistID: therapist.ID, Price: 100000}
	assert.NoError(t, db.Create(&first).Error)

	w, _, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/pricing",
		requestPath:  "/pricing",
		handler:      CreatePricing,
		body: map[string]interface{}{
			"treatment_id": treatment.ID,
			"therapist_id": therapist.ID,
			"price":        int64(200000),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListPricings_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	treatment, therapist := createPricingPrerequisites(t, db)
	assert.NoError(t, db.Create(&model.Pricing{TreatmentID: treatment.ID, TherapistID: therapist.ID, Price: 300000}).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/pricing",
		requestPath:  "/pricing?limit=10&offset=0",
		handler:      ListPricings,
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))
}

func TestGetPricingInfo_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 310000)

	status, response, err := doPricingIDRequest(r, http.MethodGet, GetPricingInfo, pricing.ID, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.True(t, response["success"].(bool))
}

func TestUpdatePricing_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 320000)

	status, response, err := doPricingIDRequest(r, http.MethodPatch, UpdatePricing, pricing.ID, map[string]interface{}{"price": int64(350000)})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.True(t, response["success"].(bool))

	var updated model.Pricing
	assert.NoError(t, db.First(&updated, pricing.ID).Error)
	assert.Equal(t, int64(350000), updated.Price)
}

func TestDeletePricing_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 330000)

	status, response, err := doPricingIDRequest(r, http.MethodDelete, DeletePricing, pricing.ID, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.True(t, response["success"].(bool))

	var deleted model.Pricing
	assert.Error(t, db.First(&deleted, pricing.ID).Error)
}
