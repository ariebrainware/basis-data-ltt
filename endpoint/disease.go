package endpoint

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ListDiseases godoc
// @Summary      List all diseases
// @Description  Get a paginated list of diseases
// @Tags         Disease
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results" default(10)
// @Param        offset query int false "Offset for pagination" default(0)
// @Success      200 {object} util.APIResponse{data=[]model.Disease} "Diseases retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /disease [get]
func ListDiseases(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	var diseases []model.Disease
	if err := db.Limit(limit).Offset(offset).Find(&diseases).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve diseases",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Diseases retrieved",
		Data: diseases,
	})
}

// helper: check whether a disease exists for a given WHERE clause
func diseaseExists(db *gorm.DB, where string, args ...interface{}) (bool, error) {
	var d model.Disease
	err := db.Where(where, args...).First(&d).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// helper: normalize inputs for create/update
func normalizeDiseaseInput(req createDiseaseRequest) (name, codename, description string) {
	name = strings.TrimSpace(req.Name)
	codename = strings.ToLower(strings.TrimSpace(req.Codename))
	description = strings.TrimSpace(req.Description)
	return
}

// helper: ensure DB is available in context or respond with server error
func ensureDB(c *gin.Context) (*gorm.DB, bool) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return nil, false
	}
	return db, true
}

// helper: get and validate id param from path
func getIDParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing disease ID",
			Err: fmt.Errorf("disease ID is required"),
		})
		return "", false
	}
	return id, true
}

// helper: fetch disease by id
func fetchDiseaseByID(db *gorm.DB, id string) (model.Disease, error) {
	var d model.Disease
	if err := db.First(&d, id).Error; err != nil {
		return model.Disease{}, err
	}
	return d, nil
}

type createDiseaseRequest struct {
	Name        string `json:"name" example:"Diabetes"`
	Codename    string `json:"codename" example:"diabetes"`
	Description string `json:"description" example:"A metabolic disease"`
}

// normalizeUpdateRequest normalizes the fields in an update request
func normalizeUpdateRequest(req *createDiseaseRequest) {
	if req.Codename != "" {
		req.Codename = strings.ToLower(strings.TrimSpace(req.Codename))
	}
	if req.Name != "" {
		req.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		req.Description = strings.TrimSpace(req.Description)
	}
}

// applyDiseaseUpdate applies the update to the existing disease
func applyDiseaseUpdate(db *gorm.DB, existing *model.Disease, updates createDiseaseRequest) error {
	return db.Model(existing).Updates(updates).Error
}

// validateRequiredFields checks that name and codename are not empty
func validateRequiredFields(c *gin.Context, name, codename string) bool {
	if name == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body: name is required",
			Err: fmt.Errorf("name is required"),
		})
		return false
	}
	if codename == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body: codename is required",
			Err: fmt.Errorf("codename is required"),
		})
		return false
	}
	return true
}

// checkDuplicateDisease checks if a disease with the given name or codename already exists
func checkDuplicateDisease(c *gin.Context, db *gorm.DB, name, codename string) bool {
	exists, err := diseaseExists(db, "LOWER(name) = ?", strings.ToLower(name))
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to check existing diseases",
			Err: err,
		})
		return false
	}
	if exists {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Disease with similar name already exists",
			Err: fmt.Errorf("disease already exists"),
		})
		return false
	}

	exists, err = diseaseExists(db, "LOWER(codename) = ?", strings.ToLower(codename))
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to check existing codenames",
			Err: err,
		})
		return false
	}
	if exists {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Disease with this codename already exists",
			Err: fmt.Errorf("codename already exists"),
		})
		return false
	}

	return true
}

// createDiseaseRecord creates a new disease record in the database
func createDiseaseRecord(db *gorm.DB, name, codename, description string) (model.Disease, error) {
	disease := model.Disease{
		Name:        name,
		Codename:    codename,
		Description: description,
	}
	err := db.Create(&disease).Error
	return disease, err
}

// CreateDisease godoc
// @Summary      Create a new disease
// @Description  Add a new disease to the system
// @Tags         Disease
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body createDiseaseRequest true "Disease information"
// @Success      200 {object} util.APIResponse{data=model.Disease} "Disease created"
// @Failure      400 {object} util.APIResponse "Invalid request"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /disease [post]
func CreateDisease(c *gin.Context) {
	var diseaseRequest createDiseaseRequest
	if err := c.ShouldBindJSON(&diseaseRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}

	db, ok := ensureDB(c)
	if !ok {
		return
	}

	name, codename, description := normalizeDiseaseInput(diseaseRequest)

	if !validateRequiredFields(c, name, codename) {
		return
	}

	if !checkDuplicateDisease(c, db, name, codename) {
		return
	}

	disease, err := createDiseaseRecord(db, name, codename, description)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create disease",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Disease created",
		Data: disease,
	})
}

// UpdateDisease godoc
// @Summary      Update disease information
// @Description  Update an existing disease's information
// @Tags         Disease
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Disease ID"
// @Param        request body createDiseaseRequest true "Updated disease information"
// @Success      200 {object} util.APIResponse{data=model.Disease} "Disease updated"
// @Failure      400 {object} util.APIResponse "Invalid request or disease not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /disease/{id} [patch]
func UpdateDisease(c *gin.Context) {
	id, ok := getIDParam(c)
	if !ok {
		return
	}

	var diseaseRequest createDiseaseRequest
	if err := c.ShouldBindJSON(&diseaseRequest); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}

	db, ok := ensureDB(c)
	if !ok {
		return
	}

	existingDisease, err := fetchDiseaseByID(db, id)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Disease not found",
			Err: err,
		})
		return
	}

	normalizeUpdateRequest(&diseaseRequest)

	if err := applyDiseaseUpdate(db, &existingDisease, diseaseRequest); err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update disease",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Disease updated",
		Data: existingDisease,
	})
}

// DeleteDisease godoc
// @Summary      Delete a disease
// @Description  Soft delete a disease by ID
// @Tags         Disease
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Disease ID"
// @Success      200 {object} util.APIResponse "Disease deleted"
// @Failure      400 {object} util.APIResponse "Disease not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /disease/{id} [delete]
func DeleteDisease(c *gin.Context) {
	id, ok := getIDParam(c)
	if !ok {
		return
	}

	db, ok := ensureDB(c)
	if !ok {
		return
	}

	existingDisease, err := fetchDiseaseByID(db, id)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Disease not found",
			Err: err,
		})
		return
	}

	if err := db.Delete(&existingDisease).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete disease",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Disease deleted",
		Data: nil,
	})
}

// GetDiseaseInfo godoc
// @Summary      Get disease information
// @Description  Get detailed information about a specific disease
// @Tags         Disease
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Disease ID"
// @Success      200 {object} util.APIResponse{data=model.Disease} "Disease retrieved"
// @Failure      400 {object} util.APIResponse "Disease not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /disease/{id} [get]
func GetDiseaseInfo(c *gin.Context) {
	id, ok := getIDParam(c)
	if !ok {
		return
	}

	db, ok := ensureDB(c)
	if !ok {
		return
	}

	existingDisease, err := fetchDiseaseByID(db, id)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Disease not found",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Disease retrieved",
		Data: existingDisease,
	})
}
