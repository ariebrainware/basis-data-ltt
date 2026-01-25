package endpoint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseQueryParams(c *gin.Context) (int, int, int, string, string, string, string) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	therapistID, _ := strconv.Atoi(c.Query("therapist_id"))
	keyword := c.Query("keyword")
	groupByDate := c.Query("group_by_date")
	sortBy := c.Query("sort")                       // supported values: full_name, patient_code
	sortDir := strings.ToLower(c.Query("sort_dir")) // supported values: asc, desc
	return limit, offset, therapistID, keyword, groupByDate, sortBy, sortDir
}

// applyCreatedAtFilter applies a created_at filter for supported ranges.
// Supported values for groupByDate: "last_2_days", "last_3_months", "last_6_months".
func applyCreatedAtFilter(query *gorm.DB, groupByDate string) *gorm.DB {
	switch groupByDate {
	case "last_2_days":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -2))
	case "last_3_months":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, -3, 0))
	case "last_6_months":
		query = query.Where("created_at >= ?", time.Now().AddDate(0, -6, 0))
	default:
		// If an unknown non-empty value is provided, log it for debugging.
		if groupByDate != "" {
			fmt.Printf("applyCreatedAtFilter: unknown group_by_date value: %s\n", groupByDate)
		}
	}
	return query
}

func fetchPatients(db *gorm.DB, limit, offset, therapistID int, keyword, groupByDate, sortBy, sortDir string) ([]model.Patient, int64, error) {
	var patients []model.Patient
	var totalPatient int64
	query := db

	// Determine order direction safely (only allow asc/desc)
	orderDir := "ASC"
	if strings.ToLower(sortDir) == "desc" {
		orderDir = "DESC"
	}

	// Apply sorting: if front-end requests sorting, use that; otherwise default to created_at DESC
	switch sortBy {
	case "full_name":
		query = query.Order(fmt.Sprintf("patients.full_name %s", orderDir))
	case "patient_code":
		query = query.Order(fmt.Sprintf("patients.patient_code %s", orderDir))
	default:
		query = query.Order("patients.created_at DESC")
	}

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("full_name LIKE ? OR patient_code LIKE ? OR address LIKE ? OR phone_number LIKE ?", kw, kw, kw, kw)
	}
	query = applyCreatedAtFilter(query, groupByDate)

	if err := query.Find(&patients).Error; err != nil {
		return nil, 0, err
	}

	db.Model(&model.Patient{}).Count(&totalPatient)
	return patients, totalPatient, nil
}

// ListPatients godoc
// @Summary      List all patients
// @Description  Get a paginated list of patients with optional filtering
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results"
// @Param        offset query int false "Offset for pagination"
// @Param        keyword query string false "Search keyword for patient name, code, address, or phone"
// @Param        group_by_date query string false "Filter by date range (last_2_days, last_3_months, last_6_months)"
// @Param        sort query string false "Optional sort field: full_name|patient_code"
// @Param        sort_dir query string false "Optional sort direction: asc|desc"
// @Success      200 {object} util.APIResponse{data=object} "Patients retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient [get]
func ListPatients(c *gin.Context) {
	limit, offset, therapistID, keyword, groupByDate, sortBy, sortDir := parseQueryParams(c)

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	patients, totalPatient, err := fetchPatients(db, limit, offset, therapistID, keyword, groupByDate, sortBy, sortDir)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve patients",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patients retrieved",
		Data: map[string]interface{}{"total": totalPatient, "total_fetched": len(patients), "patients": patients},
	})
}

type createPatientRequest struct {
	FullName       string   `json:"full_name" example:"John Doe"`
	Gender         string   `json:"gender" example:"Male"`
	Age            int      `json:"age" example:"30"`
	Job            string   `json:"job" example:"Engineer"`
	Address        string   `json:"address" example:"123 Main St"`
	PhoneNumber    []string `json:"phone_number" example:"081234567890,081234567891"`
	HealthHistory  []string `json:"health_history" example:"Diabetes,Hypertension"`
	SurgeryHistory string   `json:"surgery_history" example:"Appendectomy 2020"`
	PatientCode    string   `json:"patient_code" example:"J001"`
	Password       string   `json:"password,omitempty" example:"password123"`
	Email          string   `json:"email,omitempty" example:"john@example.com"`
}

// CreatePatient godoc
// @Summary      Create a new patient
// @Description  Register a new patient (public endpoint - no authentication required)
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Param        request body createPatientRequest true "Patient information"
// @Success      200 {object} util.APIResponse "Patient created"
// @Failure      400 {object} util.APIResponse "Invalid request or patient already exists"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient [post]
func CreatePatient(c *gin.Context) {
	patientRequest := createPatientRequest{}

	err := c.ShouldBindJSON(&patientRequest)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}
	if patientRequest.FullName == "" || len(patientRequest.PhoneNumber) == 0 {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient payload is empty or missing required fields",
			Err: fmt.Errorf("invalid payload"),
		})
		return
	}

	// Normalize full_name: trim leading/trailing whitespace and collapse internal whitespace
	patientRequest.FullName = strings.TrimSpace(patientRequest.FullName)
	// Collapse multiple internal spaces into a single space
	patientRequest.FullName = strings.Join(strings.Fields(patientRequest.FullName), " ")

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	// Prevent duplicate registration: check by full_name + any phone_number.
	// Normalize phone numbers (trim spaces, drop empties), fetch patients with the same
	// full_name, then split the stored comma-separated phone_number column and compare
	// in Go using string operations.
	normalizedPhones := []string{}
	for _, p := range patientRequest.PhoneNumber {
		ph := strings.TrimSpace(p)
		if ph != "" {
			normalizedPhones = append(normalizedPhones, ph)
		}
	}
	if len(normalizedPhones) > 0 {
		// Fetch any patients with the same full_name and perform phone matching in Go
		var matches []model.Patient
		if err := db.Where("full_name = ?", patientRequest.FullName).Find(&matches).Error; err != nil {
			util.CallServerError(c, util.APIErrorParams{
				Msg: "Failed to check existing patient",
				Err: err,
			})
			return
		}
		for _, ph := range normalizedPhones {
			for _, m := range matches {
				stored := strings.Split(m.PhoneNumber, ",")
				for _, sp := range stored {
					if strings.TrimSpace(sp) == ph {
						util.CallUserError(c, util.APIErrorParams{
							Msg: "Patient already exists with same name and phone number",
							Err: fmt.Errorf("patient duplicate detected"),
						})
						return
					}
				}
			}
		}
	}

	var existingPatient model.Patient
	var existingUser model.User

	err = db.Transaction(func(tx *gorm.DB) error {

		// Re-check for duplicate patient inside the transaction to avoid race conditions.
		if len(normalizedPhones) > 0 {
			var matchesTx []model.Patient
			if err := tx.Where("full_name = ?", patientRequest.FullName).Find(&matchesTx).Error; err != nil {
				return err
			}
			for _, ph := range normalizedPhones {
				for _, m := range matchesTx {
					stored := strings.Split(m.PhoneNumber, ",")
					for _, sp := range stored {
						if strings.TrimSpace(sp) == ph {
							// Abort the transaction if a duplicate is detected.
							return fmt.Errorf("patient already exists with same name and phone number")
						}
					}
				}
			}
		}
		// determine the patient code by fullname initials + incremented number
		initials := getInitials(patientRequest.FullName)

		// generate patient.patient_code based on patient_codes table
		var patientCodeTable model.PatientCode
		err := tx.Order("id DESC").Where("alphabet = ?", initials).First(&patientCodeTable).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("patient code not found")
			} else {
				return err
			}
		}

		var patientCode string
		if patientRequest.PatientCode != "" {
			patientCode = patientRequest.PatientCode
		} else {
			newNumber := patientCodeTable.Number + 1
			patientCode = fmt.Sprintf("%s%d", initials, newNumber)
			if err := tx.Where("alphabet = ?", initials).Updates(&model.PatientCode{
				Number:   newNumber,
				Alphabet: initials,
				Code:     patientCode,
			}).Error; err != nil {
				return err
			}
		}

		// Check if patients.patient_code already registered
		if err := tx.Where("patient_code = ?", patientCode).First(&existingPatient).Error; err == nil {
			return fmt.Errorf("patient_code already registered")
		}

		if patientRequest.Email != "" && patientRequest.Email != "-" {
			if patientRequest.Password != "" {
				// Check if users already registered
				if err := tx.Where("email = ?", patientRequest.Email).First(&existingUser).Error; err == nil {
					return fmt.Errorf("email already registered")
				}
				// Create user
				if err := tx.Create(&model.User{
					Name:     patientRequest.FullName,
					Email:    patientRequest.Email,
					Password: util.HashPassword(patientRequest.Password),
					RoleID:   2,
				}).Error; err != nil {
					return err
				}
			}
		}

		if err := tx.Create(&model.Patient{
			FullName:       patientRequest.FullName,
			Gender:         patientRequest.Gender,
			Age:            patientRequest.Age,
			Job:            patientRequest.Job,
			Address:        patientRequest.Address,
			PhoneNumber:    strings.Join(patientRequest.PhoneNumber, ","),
			PatientCode:    patientCode,
			HealthHistory:  strings.Join(patientRequest.HealthHistory, ","),
			SurgeryHistory: patientRequest.SurgeryHistory,
			Email:          patientRequest.Email,
			Password:       util.HashPassword(patientRequest.Password),
		}).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create patient",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient created",
		Data: nil,
	})
}

// UpdatePatient godoc
// @Summary      Update patient information
// @Description  Update an existing patient's information
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Patient ID"
// @Param        request body model.Patient true "Updated patient information"
// @Success      200 {object} util.APIResponse{data=model.Patient} "Patient updated"
// @Failure      400 {object} util.APIResponse "Invalid request or patient not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient/{id} [patch]
func UpdatePatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
		})
		return
	}

	patient := model.Patient{}
	if err := c.ShouldBindJSON(&patient); err != nil {
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

	var existingPatient model.Patient
	if err := db.First(&existingPatient, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient not found",
			Err: err,
		})
		return
	}

	if err := db.Model(&existingPatient).Updates(patient).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update patient",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient updated",
		Data: existingPatient,
	})
}

func getInitials(fullName string) string {
	words := strings.Fields(fullName)
	initials := ""
	if len(words) > 0 && len(words[0]) > 0 {
		initials = strings.ToUpper(string(words[0][0]))
	}
	return initials
}

func getPatientByID(c *gin.Context, db *gorm.DB) (string, model.Patient, error) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing patient ID",
			Err: fmt.Errorf("patient ID is required"),
		})
		return "", model.Patient{}, fmt.Errorf("patient ID is required")
	}

	var patient model.Patient
	if err := db.First(&patient, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Patient not found",
			Err: err,
		})
		return "", model.Patient{}, err
	}

	return id, patient, nil
}

// DeletePatient godoc
// @Summary      Delete a patient
// @Description  Soft delete a patient by ID
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Patient ID"
// @Success      200 {object} util.APIResponse "Patient deleted"
// @Failure      400 {object} util.APIResponse "Patient not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient/{id} [delete]
func DeletePatient(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	_, patient, err := getPatientByID(c, db)
	if err != nil {
		return
	}

	if err := db.Delete(&patient).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete patient",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg: "Patient deleted",
	})
}

// GetPatientInfo godoc
// @Summary      Get patient information
// @Description  Get detailed information about a specific patient
// @Tags         Patient
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Patient ID"
// @Success      200 {object} util.APIResponse{data=model.Patient} "Patient retrieved"
// @Failure      400 {object} util.APIResponse "Patient not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /patient/{id} [get]
func GetPatientInfo(c *gin.Context) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	_, patient, err := getPatientByID(c, db)
	if err != nil {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Patient retrieved",
		Data: patient,
	})
}
