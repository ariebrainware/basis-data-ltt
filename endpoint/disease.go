package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

func ListDiseases(c *gin.Context) {
	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return
	}

	var diseases []model.Disease
	if err := db.Find(&diseases).Error; err != nil {
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

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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
