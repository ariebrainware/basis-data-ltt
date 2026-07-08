package endpoint

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupEmployeeEndpointTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	r, db := setupEndpointTest(t)
	return r, db
}

func TestCreateEmployee_Success(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/employee",
		requestPath:  "/employee",
		handler:      CreateEmployee,
		body: map[string]interface{}{
			"nik":          int64(1234567890123456),
			"fullname":     "Jane Doe",
			"gender":       "Female",
			"address":      "456 Maple Rd",
			"religion":     "Christianity",
			"phone_number": "081299998888",
			"email":        "jane.doe@example.com",
			"joined_date":  "2026-05-20",
			"position":     "Staff",
			"base_salary":  6500000,
			"lunch_money":  45000,
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	var employee model.Employee
	assert.NoError(t, db.Where("nik = ?", 1234567890123456).First(&employee).Error)
	assert.Equal(t, "Jane Doe", employee.FullName)
	assert.Equal(t, "Staff", employee.Position)
	assert.Equal(t, 6500000, employee.BaseSalary)
}

func TestCreateEmployee_ValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
		msg  string
	}{
		{
			name: "missing NIK",
			body: map[string]interface{}{
				"fullname":     "Jane Doe",
				"gender":       "Female",
				"address":      "456 Maple Rd",
				"religion":     "Christianity",
				"phone_number": "081299998888",
				"email":        "jane.doe@example.com",
				"joined_date":  "2026-05-20",
				"position":     "Staff",
				"base_salary":  6500000,
				"lunch_money":  45000,
			},
			msg: "Invalid request body",
		},
		{
			name: "invalid email format",
			body: map[string]interface{}{
				"nik":          int64(1234567890123456),
				"fullname":     "Jane Doe",
				"gender":       "Female",
				"address":      "456 Maple Rd",
				"religion":     "Christianity",
				"phone_number": "081299998888",
				"email":        "invalid-email",
				"joined_date":  "2026-05-20",
				"position":     "Staff",
				"base_salary":  6500000,
				"lunch_money":  45000,
			},
			msg: "Invalid request body",
		},
		{
			name: "invalid joined_date format",
			body: map[string]interface{}{
				"nik":          int64(1234567890123456),
				"fullname":     "Jane Doe",
				"gender":       "Female",
				"address":      "456 Maple Rd",
				"religion":     "Christianity",
				"phone_number": "081299998888",
				"email":        "jane.doe@example.com",
				"joined_date":  "20-05-2026", // Invalid format
				"position":     "Staff",
				"base_salary":  6500000,
				"lunch_money":  45000,
			},
			msg: "Invalid joined_date format. Use YYYY-MM-DD or RFC3339",
		},
		{
			name: "missing position",
			body: map[string]interface{}{
				"nik":          int64(1234567890123456),
				"fullname":     "Jane Doe",
				"gender":       "Female",
				"address":      "456 Maple Rd",
				"religion":     "Christianity",
				"phone_number": "081299998888",
				"email":        "jane.doe@example.com",
				"joined_date":  "2026-05-20",
				"base_salary":  6500000,
				"lunch_money":  45000,
			},
			msg: "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupEmployeeEndpointTest(t)
			w, response, err := doRequestWithHandler(r, requestSpec{
				method:       http.MethodPost,
				registerPath: "/employee",
				requestPath:  "/employee",
				handler:      CreateEmployee,
				body:         tt.body,
			})
			assert.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.False(t, response["success"].(bool))
			assert.Contains(t, response["msg"].(string), tt.msg)
		})
	}
}

func TestCreateEmployee_DuplicateNIK(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)

	e1 := model.Employee{
		NIK:         1234567890123456,
		FullName:    "Jane Doe",
		Gender:      "Female",
		Address:     "456 Maple Rd",
		Religion:    "Christianity",
		PhoneNumber: "081299998888",
		Email:       "jane.doe@example.com",
		JoinedDate:  "2026-05-20",
		Position:    "Staff",
		BaseSalary:  6500000,
		LunchMoney:  45000,
	}
	assert.NoError(t, db.Create(&e1).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPost,
		registerPath: "/employee",
		requestPath:  "/employee",
		handler:      CreateEmployee,
		body: map[string]interface{}{
			"nik":          int64(1234567890123456), // Duplicate NIK
			"fullname":     "Another Jane",
			"gender":       "Female",
			"address":      "789 Pine Ave",
			"religion":     "Islam",
			"phone_number": "081288887777",
			"email":        "another.jane@example.com",
			"joined_date":  "2026-06-01",
			"position":     "Staff",
			"base_salary":  5000000,
			"lunch_money":  40000,
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "Employee with this NIK already exists", response["msg"].(string))
}

func TestListEmployees(t *testing.T) {
	createTestEmployees := func(t *testing.T, db *gorm.DB) {
		e1 := model.Employee{
			NIK:         1111000011110000,
			FullName:    "Alice Green",
			Gender:      "Female",
			Address:     "Green Valley",
			Religion:    "Hinduism",
			PhoneNumber: "08111111",
			Email:       "alice.green@example.com",
			JoinedDate:  "2026-01-01",
			Position:    "Staff",
			BaseSalary:  7000000,
			LunchMoney:  50000,
		}
		e2 := model.Employee{
			NIK:         2222000022220000,
			FullName:    "Bob Blue",
			Gender:      "Male",
			Address:     "Blue Ocean",
			Religion:    "Buddhism",
			PhoneNumber: "08222222",
			Email:       "bob.blue@example.com",
			JoinedDate:  "2026-02-02",
			Position:    "Manager",
			BaseSalary:  8000000,
			LunchMoney:  55000,
		}
		assert.NoError(t, db.Create(&e1).Error)
		assert.NoError(t, db.Create(&e2).Error)
	}

	t.Run("list all", func(t *testing.T) {
		r, db := setupEmployeeEndpointTest(t)
		createTestEmployees(t, db)
		w, response, err := doRequestWithHandler(r, requestSpec{
			method:       http.MethodGet,
			registerPath: "/employee",
			requestPath:  "/employee",
			handler:      ListEmployees,
		})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		data := response["data"].([]interface{})
		assert.Len(t, data, 2)
	})

	t.Run("search by name", func(t *testing.T) {
		r, db := setupEmployeeEndpointTest(t)
		createTestEmployees(t, db)
		w, response, err := doRequestWithHandler(r, requestSpec{
			method:       http.MethodGet,
			registerPath: "/employee",
			requestPath:  "/employee?keyword=Alice",
			handler:      ListEmployees,
		})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		data := response["data"].([]interface{})
		assert.Len(t, data, 1)
		first := data[0].(map[string]interface{})
		assert.Equal(t, "Alice Green", first["fullname"])
	})

	t.Run("search by NIK", func(t *testing.T) {
		r, db := setupEmployeeEndpointTest(t)
		createTestEmployees(t, db)
		w, response, err := doRequestWithHandler(r, requestSpec{
			method:       http.MethodGet,
			registerPath: "/employee",
			requestPath:  "/employee?keyword=2222000022220000",
			handler:      ListEmployees,
		})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		data := response["data"].([]interface{})
		assert.Len(t, data, 1)
		first := data[0].(map[string]interface{})
		assert.Equal(t, "Bob Blue", first["fullname"])
	})

	t.Run("search by position", func(t *testing.T) {
		r, db := setupEmployeeEndpointTest(t)
		createTestEmployees(t, db)
		w, response, err := doRequestWithHandler(r, requestSpec{
			method:       http.MethodGet,
			registerPath: "/employee",
			requestPath:  "/employee?keyword=Manager",
			handler:      ListEmployees,
		})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		data := response["data"].([]interface{})
		assert.Len(t, data, 1)
		first := data[0].(map[string]interface{})
		assert.Equal(t, "Bob Blue", first["fullname"])
		assert.Equal(t, "Manager", first["position"])
	})

	t.Run("pagination limit", func(t *testing.T) {
		r, db := setupEmployeeEndpointTest(t)
		createTestEmployees(t, db)
		w, response, err := doRequestWithHandler(r, requestSpec{
			method:       http.MethodGet,
			registerPath: "/employee",
			requestPath:  "/employee?limit=1",
			handler:      ListEmployees,
		})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		data := response["data"].([]interface{})
		assert.Len(t, data, 1)
	})
}

func TestGetEmployeeInfo_Success(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)

	employee := model.Employee{
		NIK:        123456,
		FullName:   "Test Employee",
		JoinedDate: "2026-01-01",
		Position:   "Staff",
		BaseSalary: 4000000,
		LunchMoney: 30000,
	}
	assert.NoError(t, db.Create(&employee).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/employee/:id",
		requestPath:  fmt.Sprintf("/employee/%d", employee.ID),
		handler:      GetEmployeeInfo,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "Test Employee", data["fullname"])
	assert.Equal(t, "Staff", data["position"])
}

func TestGetEmployeeInfo_NotFound(t *testing.T) {
	r, _ := setupEmployeeEndpointTest(t)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodGet,
		registerPath: "/employee/:id",
		requestPath:  "/employee/99999",
		handler:      GetEmployeeInfo,
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, response["success"].(bool))
}

func TestUpdateEmployee_Success(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)

	employee := model.Employee{
		NIK:        55555,
		FullName:   "Old Name",
		JoinedDate: "2026-01-01",
		Position:   "Staff",
		BaseSalary: 4000000,
		LunchMoney: 30000,
	}
	assert.NoError(t, db.Create(&employee).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPatch,
		registerPath: "/employee/:id",
		requestPath:  fmt.Sprintf("/employee/%d", employee.ID),
		handler:      UpdateEmployee,
		body: map[string]interface{}{
			"fullname":    "New Name",
			"position":    "Manager",
			"base_salary": 5500000,
			"joined_date": "2026-05-12",
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	var found model.Employee
	assert.NoError(t, db.First(&found, employee.ID).Error)
	assert.Equal(t, "New Name", found.FullName)
	assert.Equal(t, "Manager", found.Position)
	assert.Equal(t, 5500000, found.BaseSalary)
	assert.Equal(t, "2026-05-12", found.JoinedDate)
}

func TestUpdateEmployee_DuplicateNIK(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)

	e1 := model.Employee{
		NIK:        11111,
		FullName:   "Emp One",
		JoinedDate: "2026-01-01",
		Position:   "Staff",
	}
	e2 := model.Employee{
		NIK:        22222,
		FullName:   "Emp Two",
		JoinedDate: "2026-01-02",
		Position:   "Staff",
	}
	assert.NoError(t, db.Create(&e1).Error)
	assert.NoError(t, db.Create(&e2).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodPatch,
		registerPath: "/employee/:id",
		requestPath:  fmt.Sprintf("/employee/%d", e2.ID),
		handler:      UpdateEmployee,
		body: map[string]interface{}{
			"nik": 11111, // Try to update e2's NIK to e1's NIK
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "Another employee with this NIK already exists", response["msg"].(string))
}

func TestDeleteEmployee(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)

	employee := model.Employee{
		NIK:        999,
		FullName:   "Emp Delete",
		JoinedDate: "2026-01-01",
		Position:   "Staff",
	}
	assert.NoError(t, db.Create(&employee).Error)

	w, response, err := doRequestWithHandler(r, requestSpec{
		method:       http.MethodDelete,
		registerPath: "/employee/:id",
		requestPath:  fmt.Sprintf("/employee/%d", employee.ID),
		handler:      DeleteEmployee,
	})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	var found model.Employee
	assert.Error(t, db.First(&found, employee.ID).Error) // deleted
}
