package util

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Error   string      `json:"error"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data"`
}

type APIErrorParams struct {
	Msg string
	Err error
}

type APISuccessParams struct {
	Msg  string
	Data interface{}
}

// Contains function is to check item whether is exist or not in a list and will return bool
func Contains(d string, dl []string) bool {
	for _, v := range dl {
		if v == d {
			return true
		}
	}
	return false
}

// CallErrorNotFound is for return API response not found
func CallErrorNotFound(c *gin.Context, params APIErrorParams) {
	response := APIResponse{
		Success: false,
		Error:   params.Err.Error(),
		Msg:     params.Msg,
		Data:    map[string]interface{}{},
	}
	c.JSON(http.StatusNotFound, response)
}

// CallUserError is for return error from user side
func CallUserError(c *gin.Context, params APIErrorParams) {
	response := APIResponse{
		Success: false,
		Error:   params.Err.Error(),
		Msg:     params.Msg,
		Data:    map[string]interface{}{},
	}
	c.JSON(http.StatusBadRequest, response)
}

// CallServerError is for return API response server error
func CallServerError(c *gin.Context, params APIErrorParams) {
	response := APIResponse{
		Success: false,
		Error:   params.Err.Error(),
		Msg:     params.Msg,
		Data:    map[string]interface{}{},
	}
	c.JSON(http.StatusInternalServerError, response)
}

// CallSuccessOK is for return API response with status code 200, you need to specify msg, and data as function parameter
func CallSuccessOK(c *gin.Context, params APISuccessParams) {
	response := APIResponse{
		Success: true,
		Error:   "",
		Msg:     params.Msg,
		Data:    params.Data,
	}
	c.JSON(http.StatusOK, response)
}

// CallUserFound is for return API response with status code 307 means its redirected
func CallUserFound(c *gin.Context, params APISuccessParams) {
	response := APIResponse{
		Success: true,
		Error:   "",
		Msg:     params.Msg,
		Data:    params.Data,
	}
	c.JSON(http.StatusTemporaryRedirect, response)
}

// CallUserNotAuthorized is for return API response with status code 401 Unauthorized
func CallUserNotAuthorized(c *gin.Context, params APIErrorParams) {
	response := APIResponse{
		Success: false,
		Error:   params.Err.Error(),
		Msg:     params.Msg,
	}
	c.JSON(http.StatusUnauthorized, response)
}

// CallUserForbidden is for return API response with status code 403 Forbidden
func CallUserForbidden(c *gin.Context, params APIErrorParams) {
	response := APIResponse{
		Success: false,
		Error:   params.Err.Error(),
		Msg:     params.Msg,
		Data:    map[string]interface{}{},
	}
	c.JSON(http.StatusForbidden, response)
}

// NormalizeName normalizes a name by trimming leading/trailing whitespace
// and collapsing multiple internal spaces into single spaces.
// This ensures consistent name formatting and helps prevent duplicate detection bypass.
func NormalizeName(name string) string {
	// Trim leading and trailing whitespace
	name = strings.TrimSpace(name)
	// Collapse multiple internal spaces into single space
	return strings.Join(strings.Fields(name), " ")
}

// CalculateAge calculates age in years from a date of birth string (e.g. "YYYY-MM-DD" or RFC3339).
// If the string cannot be parsed or is in the future, it returns 0.
func CalculateAge(dobStr string) int {
	dobStr = strings.TrimSpace(dobStr)
	if dobStr == "" {
		return 0
	}

	var dob time.Time
	var err error
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05Z07:00", "02-01-2006", "02/01/2006"}
	for _, f := range formats {
		dob, err = time.Parse(f, dobStr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0
	}

	now := time.Now()
	if dob.After(now) {
		return 0
	}

	years := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}
