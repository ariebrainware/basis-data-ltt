package endpoint

import (
	"fmt"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func normalizeExpenseDate(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("date is required")
	}

	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed.Format("2006-01-02"), nil
	}

	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.Format("2006-01-02"), nil
	}

	return "", fmt.Errorf("invalid date format %q: must be YYYY-MM-DD or RFC3339", raw)
}

func getExpenseIDParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{Msg: "Missing expense ID", Err: fmt.Errorf("expense ID is required")})
		return "", false
	}
	return id, true
}

func loadExpenseOrAbort(c *gin.Context, db *gorm.DB, id string) (model.Expense, bool) {
	var expense model.Expense
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&expense).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallUserError(c, util.APIErrorParams{Msg: "Expense not found", Err: err})
			return model.Expense{}, false
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve expense", Err: err})
		return model.Expense{}, false
	}
	return expense, true
}

func applyExpenseFilters(query *gorm.DB, startDate, endDate, category, paymentMethod string) *gorm.DB {
	q := query.Where("deleted_at IS NULL")

	if startDate != "" {
		q = q.Where("expense_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("expense_date <= ?", endDate)
	}
	if category != "" {
		q = q.Where("LOWER(category) = ?", strings.ToLower(strings.TrimSpace(category)))
	}
	if paymentMethod != "" {
		q = q.Where("LOWER(payment_method) = ?", strings.ToLower(strings.TrimSpace(paymentMethod)))
	}
	return q
}

func calculateExpenseSummary(db *gorm.DB, startDate, endDate, category, paymentMethod string) (model.ExpenseSummary, error) {
	type rawCategoryRow struct {
		Category    string `gorm:"column:category"`
		TotalAmount int64  `gorm:"column:total_amount"`
		Count       int64  `gorm:"column:cnt"`
	}

	summaryQuery := applyExpenseFilters(db.Model(&model.Expense{}), startDate, endDate, category, paymentMethod)

	var rows []rawCategoryRow
	if err := summaryQuery.Select("category, SUM(amount) as total_amount, COUNT(id) as cnt").Group("category").Scan(&rows).Error; err != nil {
		return model.ExpenseSummary{}, err
	}

	var totalAmount int64
	var totalCount int64
	breakdowns := make([]model.CategoryExpenseBreakdown, 0, len(rows))

	for _, r := range rows {
		totalAmount += r.TotalAmount
		totalCount += r.Count
		breakdowns = append(breakdowns, model.CategoryExpenseBreakdown{
			Category:    r.Category,
			TotalAmount: r.TotalAmount,
			Count:       r.Count,
		})
	}

	return model.ExpenseSummary{
		TotalAmount:       totalAmount,
		TotalCount:        totalCount,
		CategoryBreakdown: breakdowns,
	}, nil
}

// ListExpenses godoc
// @Summary      List expenses
// @Description  Get a paginated list of expenses with summary analytics and optional filtering
// @Tags         Expense
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results" default(100)
// @Param        offset query int false "Offset for pagination" default(0)
// @Param        start_date query string false "Filter expenses from date (YYYY-MM-DD)"
// @Param        end_date query string false "Filter expenses to date (YYYY-MM-DD)"
// @Param        category query string false "Filter by expense category"
// @Param        payment_method query string false "Filter by payment method"
// @Success      200 {object} util.APIResponse{data=model.ListExpensesResponseData} "Expenses retrieved"
// @Failure      400 {object} util.APIResponse "Invalid query parameters"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /expense [get]
func ListExpenses(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 100, 100)
	offset := parsePositiveInt(c.Query("offset"), 0, 0)
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	category := strings.TrimSpace(c.Query("category"))
	paymentMethod := strings.TrimSpace(c.Query("payment_method"))

	if startDate != "" {
		normalized, err := normalizeExpenseDate(startDate)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid start_date format. Use YYYY-MM-DD", Err: err})
			return
		}
		startDate = normalized
	}

	if endDate != "" {
		normalized, err := normalizeExpenseDate(endDate)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid end_date format. Use YYYY-MM-DD", Err: err})
			return
		}
		endDate = normalized
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var expenses []model.Expense
	listQuery := applyExpenseFilters(db.Model(&model.Expense{}), startDate, endDate, category, paymentMethod)
	if err := listQuery.Order("expense_date DESC, id DESC").Limit(limit).Offset(offset).Find(&expenses).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve expenses", Err: err})
		return
	}

	summary, err := calculateExpenseSummary(db, startDate, endDate, category, paymentMethod)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to calculate expense summary", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg: "Expenses retrieved",
		Data: model.ListExpensesResponseData{
			Expenses: expenses,
			Summary:  summary,
		},
	})
}

// GetExpenseInfo godoc
// @Summary      Get expense information
// @Description  Retrieve an expense record by ID
// @Tags         Expense
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Expense ID"
// @Success      200 {object} util.APIResponse{data=model.Expense} "Expense retrieved"
// @Failure      400 {object} util.APIResponse "Invalid ID or expense not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /expense/{id} [get]
func GetExpenseInfo(c *gin.Context) {
	id, ok := getExpenseIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	expense, ok := loadExpenseOrAbort(c, db, id)
	if !ok {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Expense retrieved", Data: expense})
}

// CreateExpense godoc
// @Summary      Create a new expense
// @Description  Add a new expense record
// @Tags         Expense
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body model.CreateExpenseRequest true "Expense information"
// @Success      200 {object} util.APIResponse{data=model.Expense} "Expense created"
// @Failure      400 {object} util.APIResponse "Invalid request body or validation failure"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /expense [post]
func CreateExpense(c *gin.Context) {
	var req model.CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	normalizedDate, err := normalizeExpenseDate(req.ExpenseDate)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid expense_date format. Use YYYY-MM-DD or RFC3339", Err: err})
		return
	}

	category := strings.TrimSpace(req.Category)
	description := strings.TrimSpace(req.Description)
	paymentMethod := strings.TrimSpace(req.PaymentMethod)

	if category == "" || description == "" || paymentMethod == "" {
		util.CallUserError(c, util.APIErrorParams{Msg: "Category, description, and payment_method cannot be empty", Err: fmt.Errorf("missing required fields")})
		return
	}

	if req.Amount <= 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Amount must be greater than 0", Err: fmt.Errorf("amount must be > 0")})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	expense := model.Expense{
		ExpenseDate:   normalizedDate,
		Category:      category,
		Amount:        req.Amount,
		Description:   description,
		PaymentMethod: paymentMethod,
		ReceiptURL:    strings.TrimSpace(req.ReceiptURL),
		Notes:         strings.TrimSpace(req.Notes),
	}

	if err := db.Create(&expense).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to create expense", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Expense created", Data: expense})
}

// UpdateExpense godoc
// @Summary      Update expense information
// @Description  Update an existing expense record
// @Tags         Expense
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Expense ID"
// @Param        request body model.UpdateExpenseRequest true "Updated expense information"
// @Success      200 {object} util.APIResponse{data=model.Expense} "Expense updated"
// @Failure      400 {object} util.APIResponse "Invalid request or expense not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /expense/{id} [patch]
func UpdateExpense(c *gin.Context) {
	id, ok := getExpenseIDParam(c)
	if !ok {
		return
	}

	var req model.UpdateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	expense, ok := loadExpenseOrAbort(c, db, id)
	if !ok {
		return
	}

	updates := make(map[string]interface{})

	if req.ExpenseDate != nil {
		normalizedDate, err := normalizeExpenseDate(*req.ExpenseDate)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid expense_date format. Use YYYY-MM-DD or RFC3339", Err: err})
			return
		}
		updates["expense_date"] = normalizedDate
	}

	if req.Category != nil {
		cat := strings.TrimSpace(*req.Category)
		if cat == "" {
			util.CallUserError(c, util.APIErrorParams{Msg: "Category cannot be empty", Err: fmt.Errorf("invalid category")})
			return
		}
		updates["category"] = cat
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			util.CallUserError(c, util.APIErrorParams{Msg: "Amount must be greater than 0", Err: fmt.Errorf("amount must be > 0")})
			return
		}
		updates["amount"] = *req.Amount
	}

	if req.Description != nil {
		desc := strings.TrimSpace(*req.Description)
		if desc == "" {
			util.CallUserError(c, util.APIErrorParams{Msg: "Description cannot be empty", Err: fmt.Errorf("invalid description")})
			return
		}
		updates["description"] = desc
	}

	if req.PaymentMethod != nil {
		method := strings.TrimSpace(*req.PaymentMethod)
		if method == "" {
			util.CallUserError(c, util.APIErrorParams{Msg: "Payment method cannot be empty", Err: fmt.Errorf("invalid payment_method")})
			return
		}
		updates["payment_method"] = method
	}

	if req.ReceiptURL != nil {
		updates["receipt_url"] = strings.TrimSpace(*req.ReceiptURL)
	}

	if req.Notes != nil {
		updates["notes"] = strings.TrimSpace(*req.Notes)
	}

	if len(updates) == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "No fields to update", Err: fmt.Errorf("empty update payload")})
		return
	}

	if err := db.Model(&expense).Updates(updates).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update expense", Err: err})
		return
	}

	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&expense).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to reload expense", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Expense updated", Data: expense})
}

// DeleteExpense godoc
// @Summary      Delete an expense
// @Description  Soft delete an expense by ID
// @Tags         Expense
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Expense ID"
// @Success      200 {object} util.APIResponse "Expense deleted"
// @Failure      400 {object} util.APIResponse "Expense not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /expense/{id} [delete]
func DeleteExpense(c *gin.Context) {
	id, ok := getExpenseIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	expense, ok := loadExpenseOrAbort(c, db, id)
	if !ok {
		return
	}

	if err := db.Delete(&expense).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete expense", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Expense deleted", Data: nil})
}

// GetExpenseSummary godoc
// @Summary      Get expense summary statistics
// @Description  Get aggregated expense totals and category breakdowns for optional date ranges and filters
// @Tags         Expense
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        start_date query string false "Filter expenses from date (YYYY-MM-DD)"
// @Param        end_date query string false "Filter expenses to date (YYYY-MM-DD)"
// @Param        category query string false "Filter by expense category"
// @Param        payment_method query string false "Filter by payment method"
// @Success      200 {object} util.APIResponse{data=model.ExpenseSummary} "Expense summary retrieved"
// @Failure      400 {object} util.APIResponse "Invalid query parameters"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /expense/summary [get]
func GetExpenseSummary(c *gin.Context) {
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	category := strings.TrimSpace(c.Query("category"))
	paymentMethod := strings.TrimSpace(c.Query("payment_method"))

	if startDate != "" {
		normalized, err := normalizeExpenseDate(startDate)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid start_date format. Use YYYY-MM-DD", Err: err})
			return
		}
		startDate = normalized
	}

	if endDate != "" {
		normalized, err := normalizeExpenseDate(endDate)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid end_date format. Use YYYY-MM-DD", Err: err})
			return
		}
		endDate = normalized
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	summary, err := calculateExpenseSummary(db, startDate, endDate, category, paymentMethod)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to calculate expense summary", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Expense summary retrieved", Data: summary})
}
