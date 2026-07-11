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

// normalizeEmployeeDate parses and normalizes a date string to YYYY-MM-DD format
func normalizeEmployeeDate(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("date is required")
	}

	if len(raw) >= 10 {
		candidate := raw[:10]
		if _, err := time.Parse("2006-01-02", candidate); err == nil {
			return candidate, nil
		}
	}

	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.Format("2006-01-02"), nil
	}

	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return "", err
	}

	return raw, nil
}

// CreateEmployee godoc
// @Summary      Create a new employee
// @Description  Add a new employee to the system
// @Tags         Employee
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body model.CreateEmployeeRequest true "Employee information"
// @Success      200 {object} util.APIResponse{data=model.Employee} "Employee created"
// @Failure      400 {object} util.APIResponse "Invalid request body or validation failure"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /employee [post]
func CreateEmployee(c *gin.Context) {
	var req model.CreateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

	// Normalize date fields
	normalizedJoinedDate, err := normalizeEmployeeDate(req.JoinedDate)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid joined_date format. Use YYYY-MM-DD or RFC3339",
			Err: err,
		})
		return
	}

	// Trim and validate NIK before using it for duplicate checks / persistence.
	req.NIK = strings.TrimSpace(req.NIK)
	if req.NIK == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: fmt.Errorf("nik is required"),
		})
		return
	}

	// Check if NIK already exists
	var existing model.Employee
	if err := db.Unscoped().Where("nik = ?", req.NIK).First(&existing).Error; err == nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Employee with this NIK already exists",
			Err: fmt.Errorf("duplicate NIK: %s", req.NIK),
		})
		return
	} else if err != gorm.ErrRecordNotFound {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to check existing NIK",
			Err: err,
		})
		return
	}

	// Create employee record
	fullName := util.NormalizeName(req.FullName)
	gender := strings.TrimSpace(req.Gender)
	address := strings.TrimSpace(req.Address)
	religion := strings.TrimSpace(req.Religion)
	phoneNumber := strings.TrimSpace(req.PhoneNumber)
	email := strings.TrimSpace(req.Email)
	position := strings.TrimSpace(req.Position)

	if fullName == "" || gender == "" || address == "" || religion == "" || phoneNumber == "" || email == "" || position == "" {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: fmt.Errorf("missing required fields")})
		return
	}
	if req.BaseSalary < 0 || req.LunchMoney < 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: base_salary and lunch_money must be >= 0", Err: fmt.Errorf("salary must be >= 0")})
		return
	}

	employee := model.Employee{
		NIK:         req.NIK,
		FullName:    fullName,
		Gender:      gender,
		Address:     address,
		Religion:    religion,
		PhoneNumber: phoneNumber,
		Email:       email,
		JoinedDate:  normalizedJoinedDate,
		Position:    position,
		BaseSalary:  req.BaseSalary,
		LunchMoney:  req.LunchMoney,
	}
		Address:     strings.TrimSpace(req.Address),
		Religion:    strings.TrimSpace(req.Religion),
		PhoneNumber: strings.TrimSpace(req.PhoneNumber),
		Email:       strings.TrimSpace(req.Email),
		JoinedDate:  normalizedJoinedDate,
		Position:    strings.TrimSpace(req.Position),
		BaseSalary:  req.BaseSalary,
		LunchMoney:  req.LunchMoney,
	}

	if err := db.Create(&employee).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to create employee",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Employee created",
		Data: employee,
	})
}

// ListEmployees godoc
// @Summary      List all employees
// @Description  Get a paginated list of employees with optional keyword filtering
// @Tags         Employee
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results" default(100)
// @Param        offset query int false "Offset for pagination" default(0)
// @Param        keyword query string false "Search keyword for employee full_name, NIK, email, phone number, or position"
// @Success      200 {object} util.APIResponse{data=[]model.Employee} "Employees retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /employee [get]
func ListEmployees(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	keyword := c.Query("keyword")

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	query := db.Model(&model.Employee{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("full_name LIKE ? OR nik LIKE ? OR email LIKE ? OR phone_number LIKE ? OR position LIKE ?", kw, kw, kw, kw, kw)
	}

	var employees []model.Employee
	if err := query.Limit(limit).Offset(offset).Find(&employees).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to retrieve employees",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Employees retrieved",
		Data: employees,
	})
}

// GetEmployeeInfo godoc
// @Summary      Get employee details
// @Description  Retrieve details of a specific employee by ID
// @Tags         Employee
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Employee ID"
// @Success      200 {object} util.APIResponse{data=model.Employee} "Employee details retrieved"
// @Failure      400 {object} util.APIResponse "Employee not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /employee/{id} [get]
func GetEmployeeInfo(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing employee ID",
			Err: fmt.Errorf("employee ID is required"),
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

	var employee model.Employee
	if err := db.First(&employee, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{
				Msg: "Employee not found",
				Err: err,
			})
		} else {
			util.CallServerError(c, util.APIErrorParams{
				Msg: "Database error",
				Err: err,
			})
		}
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Employee retrieved",
		Data: employee,
	})
}

// UpdateEmployee godoc
// @Summary      Update employee details
// @Description  Update details of an existing employee by ID
// @Tags         Employee
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Employee ID"
// @Param        request body model.UpdateEmployeeRequest true "Updated employee information"
// @Success      200 {object} util.APIResponse{data=model.Employee} "Employee updated"
// @Failure      400 {object} util.APIResponse "Invalid request body or validation failure"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /employee/{id} [patch]
func UpdateEmployee(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing employee ID",
			Err: fmt.Errorf("employee ID is required"),
		})
		return
	}

	var req model.UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid request body",
			Err: err,
		})
		return
	}
	if req.NIK == nil && req.FullName == "" && req.Gender == "" && req.Address == "" && req.Religion == "" && req.PhoneNumber == "" && req.Email == "" && req.JoinedDate == "" && req.Position == "" && req.BaseSalary == nil && req.LunchMoney == nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "No fields to update", Err: fmt.Errorf("empty update payload")})
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

	var employee model.Employee
	if err := db.First(&employee, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{
				Msg: "Employee not found",
				Err: err,
			})
		} else {
			util.CallServerError(c, util.APIErrorParams{
				Msg: "Database error",
				Err: err,
			})
		}
		return
	}

	// Validate NIK uniqueness if updated
	if req.NIK != nil {
		var existing model.Employee
		if err := db.Unscoped().Where("nik = ? AND id != ?", *req.NIK, employee.ID).First(&existing).Error; err == nil {
			util.CallUserError(c, util.APIErrorParams{
				Msg: "Another employee with this NIK already exists",
				Err: fmt.Errorf("duplicate NIK: %s", *req.NIK),
			})
			return
		} else if err != gorm.ErrRecordNotFound {
			util.CallServerError(c, util.APIErrorParams{
				Msg: "Failed to check existing NIK",
				Err: err,
			})
			return
		}
		employee.NIK = *req.NIK
	}

	// Update fields if provided
	if req.FullName != "" {
		employee.FullName = util.NormalizeName(req.FullName)
	}
	if req.Gender != "" {
		employee.Gender = req.Gender
	}
	if req.Address != "" {
		employee.Address = strings.TrimSpace(req.Address)
	}
	if req.Religion != "" {
		employee.Religion = strings.TrimSpace(req.Religion)
	}
	if req.PhoneNumber != "" {
		employee.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	}
	if req.Email != "" {
		employee.Email = strings.TrimSpace(req.Email)
	}
	if req.JoinedDate != "" {
		normalizedJoinedDate, err := normalizeEmployeeDate(req.JoinedDate)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{
				Msg: "Invalid joined_date format. Use YYYY-MM-DD or RFC3339",
				Err: err,
			})
			return
		}
		employee.JoinedDate = normalizedJoinedDate
	}
	if req.Position != "" {
		employee.Position = strings.TrimSpace(req.Position)
	}
	if req.BaseSalary != nil {
		employee.BaseSalary = *req.BaseSalary
	}
	if req.LunchMoney != nil {
		employee.LunchMoney = *req.LunchMoney
	}

	if err := db.Save(&employee).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to update employee",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Employee updated",
		Data: employee,
	})
}

// DeleteEmployee godoc
// @Summary      Delete an employee
// @Description  Soft delete an employee by ID
// @Tags         Employee
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Employee ID"
// @Success      200 {object} util.APIResponse "Employee deleted"
// @Failure      400 {object} util.APIResponse "Employee not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /employee/{id} [delete]
func DeleteEmployee(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing employee ID",
			Err: fmt.Errorf("employee ID is required"),
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

	var employee model.Employee
	if err := db.First(&employee, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{
				Msg: "Employee not found",
				Err: err,
			})
		} else {
			util.CallServerError(c, util.APIErrorParams{
				Msg: "Database error",
				Err: err,
			})
		}
		return
	}

	if err := db.Delete(&employee).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Failed to delete employee",
			Err: err,
		})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Employee deleted",
		Data: nil,
	})
}
