package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func fetchTherapist(db *gorm.DB, limit, offset int, keyword, groupByDate string) ([]model.Therapist, int64, error) {
	var therapist []model.Therapist
	var totalTherapist int64

	query := db.Offset(offset).Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("full_name LIKE ? OR NIK LIKE ?", kw, kw)
	}
	query = applyCreatedAtFilter(query, groupByDate)

	if err := query.Find(&therapist).Error; err != nil {
		return nil, 0, err
	}

	db.Model(&model.Therapist{}).Count(&totalTherapist)
	return therapist, totalTherapist, nil
}

// ListTherapist godoc
// @Summary      List all therapists
// @Description  Get a paginated list of therapists with optional filtering
// @Tags         Therapist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results"
// @Param        offset query int false "Offset for pagination"
// @Param        keyword query string false "Search keyword for therapist name or NIK"
// @Param        group_by_date query string false "Filter by date range (last_2_days, last_3_months, last_6_months)"
// @Success      200 {object} util.APIResponse{data=object} "Therapist retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /therapist [get]
func ListTherapist(c *gin.Context) {
	limit, offset, _, keyword, groupByDate, _, _ := parseQueryParams(c)

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	therapist, totalTherapist, err := fetchTherapist(db, limit, offset, keyword, groupByDate)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve therapist",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist retrieved",
		Data: map[string]interface{}{"total": totalTherapist, "therapists": therapist},
	})
}

func getTherapistByID(c *gin.Context, db *gorm.DB) (string, model.Therapist, error) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing therapist ID",
			Err: fmt.Errorf("therapist ID is required"),
		})
		return "", model.Therapist{}, fmt.Errorf("therapist ID is required")
	}

	var therapist model.Therapist
	if err := db.First(&therapist, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Therapist not found",
			Err: err,
		})
		return "", model.Therapist{}, err
	}

	return id, therapist, nil
}

// GetTherapistInfo godoc
// @Summary      Get therapist information
// @Description  Get detailed information about a specific therapist
// @Tags         Therapist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Therapist ID"
// @Success      200 {object} util.APIResponse{data=model.Therapist} "Therapist retrieved"
// @Failure      400 {object} util.APIResponse "Therapist not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /therapist/{id} [get]
func GetTherapistInfo(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	_, therapist, err := getTherapistByID(c, db)
	if err != nil {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist retrieved",
		Data: therapist,
	})
}

type createTherapistRequest struct {
	FullName    string `json:"full_name" example:"Dr. John Smith"`
	Email       string `json:"email" example:"dr.john@example.com"`
	Password    string `json:"password" example:"password123"`
	PhoneNumber string `json:"phone_number" example:"081234567890"`
	Address     string `json:"address" example:"123 Main St"`
	DateOfBirth string `json:"date_of_birth" example:"1980-01-01"`
	NIK         string `json:"nik" example:"1234567890123456"`
	Weight      int    `json:"weight" example:"70"`
	Height      int    `json:"height" example:"175"`
	Role        string `json:"role" example:"Physical Therapist"`
	IsApproved  bool   `json:"is_approved" example:"false"`
}

func validateTherapistRequest(req createTherapistRequest) error {
	requiredFields := map[string]string{
		"FullName":    req.FullName,
		"PhoneNumber": req.PhoneNumber,
		"NIK":         req.NIK,
	}

	for fieldName, fieldValue := range requiredFields {
		if fieldValue == "" {
			return fmt.Errorf("%s is empty or missing required fields", fieldName)
		}
	}
	return nil
}

func createTherapistInDB(db *gorm.DB, req createTherapistRequest) error {
	var hashedPassword string
	if req.Password != "" {
		hashedPassword = util.HashPassword(req.Password)
	}

	var existingTherapist model.Therapist
	return db.Transaction(func(tx *gorm.DB) error {
		// Check if email and NIK already registered
		if err := tx.Where("email = ? AND NIK = ?").First(&existingTherapist).Error; err == nil {
			return fmt.Errorf("therapist already registered")
		}

		if err := tx.Create(&model.Therapist{
			FullName:    req.FullName,
			Email:       req.Email,
			Password:    hashedPassword,
			PhoneNumber: req.PhoneNumber,
			Address:     req.Address,
			DateOfBirth: req.DateOfBirth,
			NIK:         req.NIK,
			Weight:      req.Weight,
			Height:      req.Height,
			Role:        req.Role,
			IsApproved:  req.IsApproved,
		}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.User{
			Name:     req.FullName,
			Email:    req.Email,
			Password: hashedPassword,
			RoleID:   3,
		}).Error; err != nil {
			return err
		}
		return nil
	})
}

// CreateTherapist godoc
// @Summary      Create a new therapist
// @Description  Register a new therapist in the system
// @Tags         Therapist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body createTherapistRequest true "Therapist information"
// @Success      200 {object} util.APIResponse "Therapist created"
// @Failure      400 {object} util.APIResponse "Invalid request or therapist already exists"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /therapist [post]
func CreateTherapist(c *gin.Context) {
	therapistRequest := createTherapistRequest{}

	if err := c.ShouldBindJSON(&therapistRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}

	if err := validateTherapistRequest(therapistRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: err.Error(),
			Err: fmt.Errorf("invalid payload"),
		})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	if err := createTherapistInDB(db, therapistRequest); err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create therapist",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist created",
		Data: nil,
	})
}

// UpdateTherapist godoc
// @Summary      Update therapist information
// @Description  Update an existing therapist's information (excluding approval status)
// @Tags         Therapist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Therapist ID"
// @Param        request body model.Therapist true "Updated therapist information"
// @Success      200 {object} util.APIResponse "Therapist updated"
// @Failure      400 {object} util.APIResponse "Invalid request or therapist not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /therapist/{id} [patch]
func UpdateTherapist(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	id, therapist, err := getTherapistAndBindJSON(c)
	if err != nil {
		return
	}

	if therapist.IsApproved {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Cannot update therapist approval",
			Err: fmt.Errorf("cannot update therapist approval"),
		})
		return
	}

	handleTherapistUpdate(c, db, id, therapist)
}

// TherapistApproval godoc
// @Summary      Approve a therapist
// @Description  Approve a therapist account for system access
// @Tags         Therapist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Therapist ID"
// @Param        request body model.Therapist true "Therapist approval data (is_approved must be true)"
// @Success      200 {object} util.APIResponse "Therapist updated"
// @Failure      400 {object} util.APIResponse "Invalid request or approval error"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /therapist/{id} [put]
func TherapistApproval(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	handleTherapistApproval(c, db, true)
}

func handleTherapistApproval(c *gin.Context, db *gorm.DB, isApproval bool) {
	id, therapist, err := getTherapistAndBindJSON(c)
	if err != nil {
		return
	}

	if isApproval && !therapist.IsApproved {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Changes allowed only for approval and it must be true",
			Err: fmt.Errorf("misinterpretation of request"),
		})
		return
	}

	handleTherapistUpdate(c, db, id, therapist)
}

func handleTherapistUpdate(c *gin.Context, db *gorm.DB, id string, therapist model.Therapist) {
	if err := updateTherapistInDB(db, id, therapist); err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update therapist",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist updated",
		Data: nil,
	})
}

func updateTherapistInDB(db *gorm.DB, id string, therapist model.Therapist) error {
	var existingTherapist model.Therapist
	if err := db.Where("id = ?", id).First(&existingTherapist, id).Error; err != nil {
		return err
	}

	if err := db.Model(&existingTherapist).Updates(therapist).Error; err != nil {
		return err
	}

	return nil
}

func getTherapistAndBindJSON(c *gin.Context) (string, model.Therapist, error) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing therapist ID",
			Err: fmt.Errorf("therapist ID is required"),
		})
		return "", model.Therapist{}, fmt.Errorf("therapist ID is required")
	}

	therapist := model.Therapist{}
	if err := c.ShouldBindJSON(&therapist); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return "", model.Therapist{}, err
	}

	return id, therapist, nil
}

// DeleteTherapist godoc
// @Summary      Delete a therapist
// @Description  Soft delete a therapist by ID
// @Tags         Therapist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Therapist ID"
// @Success      200 {object} util.APIResponse "Therapist deleted"
// @Failure      400 {object} util.APIResponse "Therapist not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /therapist/{id} [delete]
func DeleteTherapist(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing therapist ID",
			Err: fmt.Errorf("therapist ID is required"),
		})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	var existingTherapist model.Therapist
	if err := db.First(&existingTherapist, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Therapist not found",
			Err: err,
		})
		return
	}

	if err := db.Delete(&existingTherapist).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete therapist",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist deleted",
		Data: nil,
	})
}
