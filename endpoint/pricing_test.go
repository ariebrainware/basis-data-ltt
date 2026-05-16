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

func createPricingPrerequisites(t *testing.T, db *gorm.DB) model.Therapist {
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

	return therapist
}

func createPricingRecordForTest(t *testing.T, db *gorm.DB, price int64) model.Pricing {
	t.Helper()
	therapist := createPricingPrerequisites(t, db)
	pricing := model.Pricing{TherapistID: therapist.ID, Price: price}
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
	therapist := createPricingPrerequisites(t, db)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/pricing",
		requestPath:  "/pricing",
		handler:      CreatePricing,
		body: map[string]interface{}{
			"therapist_id": therapist.ID,
			"price":        int64(275000),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	var pricing model.Pricing
	assert.NoError(t, db.Where("therapist_id = ?", therapist.ID).First(&pricing).Error)
	assert.Equal(t, int64(275000), pricing.Price)
}

func TestCreatePricing_TherapistNotRegistered(t *testing.T) {
	r, _ := setupPricingEndpointTest(t)

	w, _, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/pricing",
		requestPath:  "/pricing",
		handler:      CreatePricing,
		body: map[string]interface{}{
			"therapist_id": uint(999999),
			"price":        int64(275000),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePricing_AllowMultipleRecordsPerTherapist(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	therapist := createPricingPrerequisites(t, db)

	first := model.Pricing{TherapistID: therapist.ID, Price: 100000}
	assert.NoError(t, db.Create(&first).Error)

	w, _, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/pricing",
		requestPath:  "/pricing",
		handler:      CreatePricing,
		body: map[string]interface{}{
			"therapist_id": therapist.ID,
			"price":        int64(200000),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var count int64
	assert.NoError(t, db.Model(&model.Pricing{}).Where("therapist_id = ?", therapist.ID).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestListPricings_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	therapist := createPricingPrerequisites(t, db)
	assert.NoError(t, db.Create(&model.Pricing{TherapistID: therapist.ID, Price: 300000}).Error)

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

func TestListPricings_ExcludeOrphanPricing(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	therapist := createPricingPrerequisites(t, db)
	assert.NoError(t, db.Create(&model.Pricing{TherapistID: therapist.ID, Price: 300000}).Error)
	assert.NoError(t, db.Delete(&therapist).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/pricing",
		requestPath:  "/pricing?limit=10&offset=0",
		handler:      ListPricings,
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	data, ok := response["data"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, data, 0)
}

func TestGetPricingInfo_Success(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 310000)

	status, response, err := doPricingIDRequest(r, http.MethodGet, GetPricingInfo, pricing.ID, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.True(t, response["success"].(bool))

	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Pricing Therapist", data["therapist_name"])
}

func TestGetPricingInfo_TherapistNotRegistered(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 310000)

	var therapist model.Therapist
	assert.NoError(t, db.First(&therapist, pricing.TherapistID).Error)
	assert.NoError(t, db.Delete(&therapist).Error)

	status, _, err := doPricingIDRequest(r, http.MethodGet, GetPricingInfo, pricing.ID, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, status)
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

func TestUpdatePricing_TherapistNotRegistered(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 320000)

	status, _, err := doPricingIDRequest(r, http.MethodPatch, UpdatePricing, pricing.ID, map[string]interface{}{"therapist_id": uint(999999)})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, status)
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

func TestDeletePricing_TherapistNotRegistered(t *testing.T) {
	r, db := setupPricingEndpointTest(t)
	pricing := createPricingRecordForTest(t, db, 330000)

	var therapist model.Therapist
	assert.NoError(t, db.First(&therapist, pricing.TherapistID).Error)
	assert.NoError(t, db.Delete(&therapist).Error)

	status, _, err := doPricingIDRequest(r, http.MethodDelete, DeletePricing, pricing.ID, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, status)
}
