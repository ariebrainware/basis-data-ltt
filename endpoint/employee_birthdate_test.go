package endpoint

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validEmployeeBody(birthDate interface{}) map[string]interface{} {
	body := map[string]interface{}{
		"nik":          "3201000000009999",
		"full_name":    "Fara Nadya",
		"gender":       "Female",
		"address":      "Kampung Aceh 174, Muka Kuning",
		"religion":     "Islam",
		"phone_number": "083180662979",
		"email":        "fara@example.com",
		"joined_date":  "2026-02-13",
		"position":     "Staff",
		"base_salary":  3000000,
		"lunch_money":  10000,
	}
	if birthDate != nil {
		body["birth_date"] = birthDate
	}
	return body
}

func TestCreateEmployee_BirthDate(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)
	r.POST("/employee", CreateEmployee)

	w, response, err := performRequest(r, requestSpec{
		method: http.MethodPost, requestPath: "/employee",
		body: validEmployeeBody("  Medan, 20 Juli 2007  "),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Medan, 20 Juli 2007", response["data"].(map[string]interface{})["birth_date"])

	var saved model.Employee
	require.NoError(t, db.Where("nik = ?", "3201000000009999").First(&saved).Error)
	assert.Equal(t, "Medan, 20 Juli 2007", saved.BirthDate)
}

func TestCreateEmployee_BirthDateOptional(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)
	r.POST("/employee", CreateEmployee)

	w, _, err := performRequest(r, requestSpec{
		method: http.MethodPost, requestPath: "/employee", body: validEmployeeBody(nil),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var saved model.Employee
	require.NoError(t, db.Where("nik = ?", "3201000000009999").First(&saved).Error)
	assert.Empty(t, saved.BirthDate)
}

func TestCreateEmployee_BirthDateTooLong(t *testing.T) {
	r, _ := setupEmployeeEndpointTest(t)
	r.POST("/employee", CreateEmployee)

	w, response, err := performRequest(r, requestSpec{
		method: http.MethodPost, requestPath: "/employee",
		body: validEmployeeBody(strings.Repeat("a", 201)),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Invalid request body", response["msg"])
}

func TestUpdateEmployee_BirthDate(t *testing.T) {
	r, db := setupEmployeeEndpointTest(t)
	r.PATCH("/employee/:id", UpdateEmployee)

	employee := model.Employee{
		NIK: "3201000000008888", FullName: "Jane Doe", Gender: "Female", Address: "Jl. Mawar",
		Religion: "Islam", PhoneNumber: "0812", Email: "jane@example.com",
		JoinedDate: "2026-01-01", Position: "Staff", BaseSalary: 1, LunchMoney: 1,
	}
	require.NoError(t, db.Create(&employee).Error)
	path := fmt.Sprintf("/employee/%d", employee.ID)

	// birth_date alone is a valid update
	w, _, err := performRequest(r, requestSpec{
		method: http.MethodPatch, requestPath: path,
		body: map[string]interface{}{"birth_date": "Batam, 1 Januari 2000"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&employee, employee.ID).Error)
	assert.Equal(t, "Batam, 1 Januari 2000", employee.BirthDate)

	// Other updates leave it untouched
	w, _, err = performRequest(r, requestSpec{
		method: http.MethodPatch, requestPath: path,
		body: map[string]interface{}{"position": "Admin"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, db.First(&employee, employee.ID).Error)
	assert.Equal(t, "Batam, 1 Januari 2000", employee.BirthDate)

	// Empty string clears it
	w, _, err = performRequest(r, requestSpec{
		method: http.MethodPatch, requestPath: path,
		body: map[string]interface{}{"birth_date": ""},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, db.First(&employee, employee.ID).Error)
	assert.Empty(t, employee.BirthDate)
}
