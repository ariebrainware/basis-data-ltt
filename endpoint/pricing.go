package endpoint

import (
	"fmt"
	"strconv"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createPricingRequest struct {
	TreatmentID uint  `json:"treatment_id"`
	TherapistID uint  `json:"therapist_id"`
	Price       int64 `json:"price"`
}

type updatePricingRequest struct {
	TreatmentID *uint  `json:"treatment_id"`
	TherapistID *uint  `json:"therapist_id"`
	Price       *int64 `json:"price"`
}

func getPricingIDParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing pricing ID",
			Err: fmt.Errorf("pricing ID is required"),
		})
		return "", false
	}
	return id, true
}

func validateCreatePricingInput(c *gin.Context, req createPricingRequest) bool {
	if req.TreatmentID == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: treatment_id is required", Err: fmt.Errorf("treatment_id is required")})
		return false
	}
	if req.TherapistID == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: therapist_id is required", Err: fmt.Errorf("therapist_id is required")})
		return false
	}
	if req.Price < 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: price must be >= 0", Err: fmt.Errorf("price must be >= 0")})
		return false
	}
	return true
}

func ensurePricingRelationsExist(c *gin.Context, db *gorm.DB, treatmentID, therapistID uint) bool {
	var treatment model.Treatment
	if err := db.First(&treatment, treatmentID).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Treatment not found", Err: err})
		return false
	}

	var therapist model.Therapist
	if err := db.First(&therapist, therapistID).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Therapist not found", Err: err})
		return false
	}

	return true
}

func pricingExistsForTreatment(db *gorm.DB, treatmentID uint, excludeID uint) (bool, error) {
	q := db.Where("treatment_id = ?", treatmentID)
	if excludeID != 0 {
		q = q.Where("id != ?", excludeID)
	}

	var pricing model.Pricing
	err := q.First(&pricing).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListPricings godoc
// @Summary      List all pricings
// @Description  Get a paginated list of pricing records
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results" default(100)
// @Param        offset query int false "Offset for pagination" default(0)
// @Success      200 {object} util.APIResponse{data=[]model.Pricing} "Pricings retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /pricing [get]
func ListPricings(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var pricings []model.Pricing
	if err := db.Order("id DESC").Limit(limit).Offset(offset).Find(&pricings).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve pricings", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricings retrieved", Data: pricings})
}

// GetPricingInfo godoc
// @Summary      Get pricing information
// @Description  Retrieve a pricing record by ID
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Pricing ID"
// @Success      200 {object} util.APIResponse{data=model.Pricing} "Pricing retrieved"
// @Failure      400 {object} util.APIResponse "Invalid ID or pricing not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /pricing/{id} [get]
func GetPricingInfo(c *gin.Context) {
	id, ok := getPricingIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var pricing model.Pricing
	if err := db.First(&pricing, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Pricing not found", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricing retrieved", Data: pricing})
}

// CreatePricing godoc
// @Summary      Create a new pricing
// @Description  Add a new pricing record for a treatment
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body createPricingRequest true "Pricing information"
// @Success      200 {object} util.APIResponse{data=model.Pricing} "Pricing created"
// @Failure      400 {object} util.APIResponse "Invalid request"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /pricing [post]
func CreatePricing(c *gin.Context) {
	var req createPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	if !validateCreatePricingInput(c, req) {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	if !ensurePricingRelationsExist(c, db, req.TreatmentID, req.TherapistID) {
		return
	}

	exists, err := pricingExistsForTreatment(db, req.TreatmentID, 0)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to check existing pricing", Err: err})
		return
	}
	if exists {
		util.CallUserError(c, util.APIErrorParams{Msg: "Pricing for this treatment already exists", Err: fmt.Errorf("duplicate treatment_id")})
		return
	}

	pricing := model.Pricing{TreatmentID: req.TreatmentID, TherapistID: req.TherapistID, Price: req.Price}
	if err := db.Create(&pricing).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to create pricing", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricing created", Data: pricing})
}

// UpdatePricing godoc
// @Summary      Update pricing information
// @Description  Update an existing pricing record
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Pricing ID"
// @Param        request body updatePricingRequest true "Updated pricing information"
// @Success      200 {object} util.APIResponse{data=model.Pricing} "Pricing updated"
// @Failure      400 {object} util.APIResponse "Invalid request or pricing not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /pricing/{id} [patch]
func UpdatePricing(c *gin.Context) {
	id, ok := getPricingIDParam(c)
	if !ok {
		return
	}

	var req updatePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var pricing model.Pricing
	if err := db.First(&pricing, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Pricing not found", Err: err})
		return
	}

	updates := map[string]interface{}{}

	if req.TreatmentID != nil {
		if *req.TreatmentID == 0 {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: treatment_id must be > 0", Err: fmt.Errorf("invalid treatment_id")})
			return
		}
		exists, err := pricingExistsForTreatment(db, *req.TreatmentID, pricing.ID)
		if err != nil {
			util.CallServerError(c, util.APIErrorParams{Msg: "Failed to check existing pricing", Err: err})
			return
		}
		if exists {
			util.CallUserError(c, util.APIErrorParams{Msg: "Pricing for this treatment already exists", Err: fmt.Errorf("duplicate treatment_id")})
			return
		}

		var treatment model.Treatment
		if err := db.First(&treatment, *req.TreatmentID).Error; err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Treatment not found", Err: err})
			return
		}

		updates["treatment_id"] = *req.TreatmentID
	}

	if req.TherapistID != nil {
		if *req.TherapistID == 0 {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: therapist_id must be > 0", Err: fmt.Errorf("invalid therapist_id")})
			return
		}

		var therapist model.Therapist
		if err := db.First(&therapist, *req.TherapistID).Error; err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Therapist not found", Err: err})
			return
		}

		updates["therapist_id"] = *req.TherapistID
	}

	if req.Price != nil {
		if *req.Price < 0 {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: price must be >= 0", Err: fmt.Errorf("invalid price")})
			return
		}
		updates["price"] = *req.Price
	}

	if len(updates) == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "No fields to update", Err: fmt.Errorf("empty update payload")})
		return
	}

	if err := db.Model(&pricing).Updates(updates).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update pricing", Err: err})
		return
	}

	if err := db.First(&pricing, pricing.ID).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to reload pricing", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricing updated", Data: pricing})
}

// DeletePricing godoc
// @Summary      Delete a pricing
// @Description  Soft delete a pricing by ID
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Pricing ID"
// @Success      200 {object} util.APIResponse "Pricing deleted"
// @Failure      400 {object} util.APIResponse "Pricing not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /pricing/{id} [delete]
func DeletePricing(c *gin.Context) {
	id, ok := getPricingIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var pricing model.Pricing
	if err := db.First(&pricing, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Pricing not found", Err: err})
		return
	}

	if err := db.Delete(&pricing).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete pricing", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricing deleted", Data: nil})
}
