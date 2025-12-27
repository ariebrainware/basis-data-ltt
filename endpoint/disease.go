package endpoint

import (
	"fmt"
	"strconv"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

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
	Name        string `json:"name"`
	Description string `json:"description"`
}

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
