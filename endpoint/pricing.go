package endpoint

import (
	"fmt"
	"strconv"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createPricingRequest struct {
	TherapistID uint  `json:"therapist_id"`
	Price       int64 `json:"price"`
}

type updatePricingRequest struct {
	TherapistID *uint  `json:"therapist_id"`
	Price       *int64 `json:"price"`
}

type pricingWithTherapist struct {
	model.Pricing
	TherapistName string `json:"therapist_name" gorm:"column:therapist_name"`
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

func ensureTherapistRegistered(db *gorm.DB, therapistID uint) error {
	var therapist model.Therapist
	if err := db.First(&therapist, therapistID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("therapist_id %d is not registered", therapistID)
		}
		return err
	}
	return nil
}

func ensurePricingTherapistRegistered(db *gorm.DB, pricing model.Pricing) error {
	return ensureTherapistRegistered(db, pricing.TherapistID)
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

	var pricings []pricingWithTherapist
	if err := db.Table("pricings").
		Select("pricings.*, therapists.full_name as therapist_name").
		Joins("JOIN therapists ON therapists.id = pricings.therapist_id").
		Where("therapists.deleted_at IS NULL AND pricings.deleted_at IS NULL").
		Order("pricings.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&pricings).Error; err != nil {
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
// @Success      200 {object} util.APIResponse{data=pricingWithTherapist} "Pricing retrieved"
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

	var pricingInfo pricingWithTherapist
	if err := db.Table("pricings").
		Select("pricings.*, therapists.full_name as therapist_name").
		Joins("JOIN therapists ON therapists.id = pricings.therapist_id").
		Where("pricings.id = ? AND therapists.deleted_at IS NULL AND pricings.deleted_at IS NULL", id).
		First(&pricingInfo).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Therapist not found", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricing retrieved", Data: pricingInfo})
}

func resolveTherapistID(c *gin.Context, db *gorm.DB, req model.TreatementRequest) (uint, error) {
	if roleID, ok := middleware.GetRoleID(c); ok && roleID == model.RoleTherapist {
		return getTherapistIDFromSession(db, c.GetHeader("session-token"))
	}

	if req.TherapistID == 0 {
		return 0, fmt.Errorf("therapist id is required")
	}

	return req.TherapistID, nil
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

	if err := ensureTherapistRegistered(db, req.TherapistID); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Therapist not found", Err: err})
		return
	}

	pricing := model.Pricing{TherapistID: req.TherapistID, Price: req.Price}
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

	if req.TherapistID != nil {
		if *req.TherapistID == 0 {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: therapist_id must be > 0", Err: fmt.Errorf("invalid therapist_id")})
			return
		}

		if err := ensureTherapistRegistered(db, *req.TherapistID); err != nil {
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

	if err := ensurePricingTherapistRegistered(db, pricing); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Therapist not found", Err: err})
		return
	}

	if err := db.Delete(&pricing).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete pricing", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Pricing deleted", Data: nil})
}
