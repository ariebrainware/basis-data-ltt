package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

func ListDeceases(c *gin.Context) {
	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return
	}

	var deceases []model.Patient
	if err := db.Find(&deceases).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve deceases",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Deceases retrieved",
		Data: deceases,
	})
}

type createDeceaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func CreateDecease(c *gin.Context) {
	deceaseRequest := createDeceaseRequest{}

	err := c.ShouldBindJSON(&deceaseRequest)
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

	decease := model.Decease{
		Name:        deceaseRequest.Name,
		Description: deceaseRequest.Description,
	}
	if err := db.Create(&decease).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create decease",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Decease created",
		Data: decease,
	})
}

func UpdateDecease(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing decease ID",
			Err: fmt.Errorf("decease ID is required"),
		})
		return
	}

	deceaseRequest := createDeceaseRequest{}

	err := c.ShouldBindJSON(&deceaseRequest)
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

	var existingDecease model.Decease
	if err := db.First(&existingDecease, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Decease not found",
			Err: err,
		})
		return
	}

	if err := db.Model(&existingDecease).Updates(deceaseRequest).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update decease",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Decease updated",
		Data: existingDecease,
	})
}

func DeleteDecease(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing decease ID",
			Err: fmt.Errorf("decease ID is required"),
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

	var existingDecease model.Decease
	if err := db.First(&existingDecease, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Decease not found",
			Err: err,
		})
		return
	}

	if err := db.Delete(&existingDecease).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete decease",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Decease deleted",
		Data: nil,
	})
}

func GetDeceaseInfo(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing decease ID",
			Err: fmt.Errorf("decease ID is required"),
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

	var existingDecease model.Decease
	if err := db.First(&existingDecease, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Decease not found",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Decease retrieved",
		Data: existingDecease,
	})
}
