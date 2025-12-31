package endpoint

import (
	"fmt"
	"strconv"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
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

type createDiseaseRequest struct {
	Name        string `json:"name" example:"Diabetes"`
	Description string `json:"description" example:"A metabolic disease"`
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
	diseaseRequest := createDiseaseRequest{}

	err := c.ShouldBindJSON(&diseaseRequest)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
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

	disease := model.Disease{
		Name:        diseaseRequest.Name,
		Description: diseaseRequest.Description,
	}
	if err := db.Create(&disease).Error; err != nil {
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
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing disease ID",
			Err: fmt.Errorf("disease ID is required"),
		})
		return
	}

	diseaseRequest := createDiseaseRequest{}

	err := c.ShouldBindJSON(&diseaseRequest)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
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

	var existingDisease model.Disease
	if err := db.First(&existingDisease, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Disease not found",
			Err: err,
		})
		return
	}

	if err := db.Model(&existingDisease).Updates(diseaseRequest).Error; err != nil {
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
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing disease ID",
			Err: fmt.Errorf("disease ID is required"),
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

	var existingDisease model.Disease
	if err := db.First(&existingDisease, id).Error; err != nil {
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
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing disease ID",
			Err: fmt.Errorf("disease ID is required"),
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

	var existingDisease model.Disease
	if err := db.First(&existingDisease, id).Error; err != nil {
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
