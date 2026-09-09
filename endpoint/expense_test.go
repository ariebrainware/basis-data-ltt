package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupExpenseTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	r, _ := setupEndpointTest(t)
	r.GET("/expense", ListExpenses)
	r.GET("/expense/summary", GetExpenseSummary)
	r.POST("/expense", CreateExpense)
	r.GET("/expense/:id", GetExpenseInfo)
	r.PATCH("/expense/:id", UpdateExpense)
	r.DELETE("/expense/:id", DeleteExpense)
	return r
}

func TestExpenseCRUDFlow(t *testing.T) {
	r := setupExpenseTestRouter(t)

	// 1. Create an expense
	createPayload := `{"expense_date":"2025-01-15","category":"Operational","amount":150000,"description":"Electricity bill","payment_method":"bank_transfer","notes":"Paid on time"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/expense", bytes.NewReader([]byte(createPayload)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var createResp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &createResp)
	assert.NoError(t, err)
	data, ok := createResp["data"].(map[string]interface{})
	assert.True(t, ok)
	expenseID := fmt.Sprintf("%.0f", data["ID"].(float64))
	assert.NotEmpty(t, expenseID)

	// 2. Fetch the created expense
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/expense/"+expenseID, nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var getResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &getResp)
	assert.NoError(t, err)
	getData := getResp["data"].(map[string]interface{})
	assert.Equal(t, "Operational", getData["category"])
	assert.Equal(t, float64(150000), getData["amount"])
	assert.Equal(t, "Electricity bill", getData["description"])

	// 3. Update the expense
	updatePayload := `{"amount":175000,"description":"Electricity bill updated"}`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/expense/"+expenseID, bytes.NewReader([]byte(updatePayload)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var updateResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &updateResp)
	assert.NoError(t, err)
	updatedData := updateResp["data"].(map[string]interface{})
	assert.Equal(t, float64(175000), updatedData["amount"])
	assert.Equal(t, "Electricity bill updated", updatedData["description"])

	// 4. List expenses
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/expense?limit=10&offset=0", nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var listResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	listData := listResp["data"].(map[string]interface{})
	expensesList := listData["expenses"].([]interface{})
	assert.Len(t, expensesList, 1)
	summary := listData["summary"].(map[string]interface{})
	assert.Equal(t, float64(175000), summary["total_amount"])
	assert.Equal(t, float64(1), summary["total_count"])

	// 5. Delete the expense
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/expense/"+expenseID, nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 6. Fetching deleted expense should fail (soft deleted)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/expense/"+expenseID, nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestExpenseFiltersAndSummary(t *testing.T) {
	r := setupExpenseTestRouter(t)

	expenses := []model.CreateExpenseRequest{
		{ExpenseDate: "2025-01-10", Category: "Operational", Amount: 100000, Description: "Water", PaymentMethod: "cash"},
		{ExpenseDate: "2025-01-15", Category: "Supplies", Amount: 200000, Description: "Gloves", PaymentMethod: "bank_transfer"},
		{ExpenseDate: "2025-02-01", Category: "Supplies", Amount: 300000, Description: "Bandages", PaymentMethod: "cash"},
	}

	for _, exp := range expenses {
		b, _ := json.Marshal(exp)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/expense", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// Test category filter
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/expense?category=Supplies", nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var listResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	listData := listResp["data"].(map[string]interface{})
	expensesList := listData["expenses"].([]interface{})
	assert.Len(t, expensesList, 2)
	summary := listData["summary"].(map[string]interface{})
	assert.Equal(t, float64(500000), summary["total_amount"])

	// Test date range filter
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/expense?start_date=2025-01-01&end_date=2025-01-31", nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	listData = listResp["data"].(map[string]interface{})
	expensesList = listData["expenses"].([]interface{})
	assert.Len(t, expensesList, 2)

	// Test standalone summary endpoint
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/expense/summary?payment_method=cash", nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var summaryResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &summaryResp)
	summaryData := summaryResp["data"].(map[string]interface{})
	assert.Equal(t, float64(400000), summaryData["total_amount"])
	assert.Equal(t, float64(2), summaryData["total_count"])
}

func TestExpenseValidationFailures(t *testing.T) {
	r := setupExpenseTestRouter(t)

	// 1. Missing required fields
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/expense", bytes.NewReader([]byte(`{"expense_date":"2025-01-15"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 2. Negative/zero amount
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/expense", bytes.NewReader([]byte(`{"expense_date":"2025-01-15","category":"Operational","amount":0,"description":"Test","payment_method":"cash"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 3. Invalid date format
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/expense", bytes.NewReader([]byte(`{"expense_date":"invalid-date","category":"Operational","amount":1000,"description":"Test","payment_method":"cash"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 4. Invalid start_date in list query
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/expense?start_date=invalid-date", nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 5. Update non-existent record
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/expense/99999", bytes.NewReader([]byte(`{"amount":1000}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 6. Update with empty payload
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/expense/1", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
