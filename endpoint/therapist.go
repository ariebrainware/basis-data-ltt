package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func fetchTherapist(limit, offset int, keyword, groupByDate string) ([]model.Therapist, int64, error) {
	var therapist []model.Therapist
	var totalTherapist int64

	db, err := config.ConnectMySQL()
	if err != nil {
		return nil, 0, err
	}

	query := db.Offset(offset).Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if keyword != "" {
		query = query.Where("full_name LIKE ? OR NIK LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	query = applyGroupByDateFilter(query, groupByDate)

	if err := query.Find(&therapist).Error; err != nil {
		return nil, 0, err
	}

	db.Model(&model.Therapist{}).Count(&totalTherapist)
	return therapist, totalTherapist, nil
}

func ListTherapist(c *gin.Context) {
	limit, offset, _, keyword, groupByDate := parseQueryParams(c)

	therapist, totalTherapist, err := fetchTherapist(limit, offset, keyword, groupByDate)
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

func getTherapistByID(c *gin.Context) (string, *gorm.DB, model.Therapist, error) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing therapist ID",
			Err: fmt.Errorf("therapist ID is required"),
		})
		return "", nil, model.Therapist{}, fmt.Errorf("therapist ID is required")
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
		})
		return "", nil, model.Therapist{}, err
	}

	var therapist model.Therapist
	if err := db.First(&therapist, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Therapist not found",
			Err: err,
		})
		return "", nil, model.Therapist{}, err
	}

	return id, db, therapist, nil
}

func GetTherapistInfo(c *gin.Context) {
	_, _, therapist, err := getTherapistByID(c)
	if err != nil {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Therapist retrieved",
		Data: therapist,
	})
}

type createTherapistRequest struct {
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	PhoneNumber string `json:"phone_number"`
	Address     string `json:"address"`
	DateOfBirth string `json:"date_of_birth"`
	NIK         string `json:"nik"`
	Weight      int    `json:"weight"`
	Height      int    `json:"height"`
	Role        string `json:"role"`
	IsApproved  bool   `json:"is_approved"`
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

	db, err := config.ConnectMySQL()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to connect to MySQL",
			Err: err,
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

func UpdateTherapist(c *gin.Context) {
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

	handleTherapistUpdate(c, id, therapist)
}

func TherapistApproval(c *gin.Context) {
	handleTherapistApproval(c, true)
}

func handleTherapistApproval(c *gin.Context, isApproval bool) {
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

	handleTherapistUpdate(c, id, therapist)
}

func handleTherapistUpdate(c *gin.Context, id string, therapist model.Therapist) {
	if err := updateTherapistInDB(id, therapist); err != nil {
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

func updateTherapistInDB(id string, therapist model.Therapist) error {
	db, err := config.ConnectMySQL()
	if err != nil {
		return err
	}

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

func DeleteTherapist(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing therapist ID",
			Err: fmt.Errorf("therapist ID is required"),
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
