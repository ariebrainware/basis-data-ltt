package endpoint

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

type updateTransactionRequest struct {
	Amount        *int64  `json:"amount"`
	Remarks       *string `json:"remarks"`
	PaymentMethod *string `json:"payment_method"`
	PaymentStatus *string `json:"payment_status"`
}

func getTransactionIDParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Missing transaction ID",
			Err: fmt.Errorf("transaction ID is required"),
		})
		return "", false
	}
	return id, true
}

func getTransactionDateFilter(c *gin.Context) (string, error) {
	raw := c.Query("date")
	if raw == "" {
		raw = c.Query("limiter")
	}
	if raw == "" {
		raw = c.Query("treatment_date")
	}
	if raw == "" {
		raw = c.Query("group_by_date")
	}
	if raw == "" {
		return "", nil
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

// ListTransactions godoc
// @Summary      List all transactions
// @Description  Get a paginated list of transaction records
// @Tags         Transaction
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results" default(100)
// @Param        offset query int false "Offset for pagination" default(0)
// @Param        date query string false "Filter by treatment date (YYYY-MM-DD). Also accepts RFC3339 datetime."
// @Param        limiter query string false "Alias for date filter"
// @Param        treatment_date query string false "Alias for date filter"
// @Param        group_by_date query string false "Alias for date filter"
// @Success      200 {object} util.APIResponse{data=model.ListTransactionsResponseData} "Transactions retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /transaction [get]
func ListTransactions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	dateFilter, err := getTransactionDateFilter(c)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid date format. Use YYYY-MM-DD", Err: err})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var transactions []model.ListTransactionResponse
	txQuery := db.Model(&model.Transaction{}).
		Select("transactions.*, patients.full_name as patient_name, treatments.treatment_date").
		Joins("LEFT JOIN treatments ON treatments.id = transactions.treatment_id").
		Joins("LEFT JOIN patients ON patients.patient_code = treatments.patient_code")

	if dateFilter != "" {
		txQuery = txQuery.Where("DATE(treatments.treatment_date) = ?", dateFilter)
	}

	if err := txQuery.
		Order("transactions.id DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve transactions", Err: err})
		return
	}

	// Calculate summary
	targetDate := time.Now().Format("2006-01-02")
	if dateFilter != "" {
		targetDate = dateFilter
	}

	// Total amount for the target date
	var totalAmount int64
	if err := db.Model(&model.Transaction{}).
		Select("COALESCE(SUM(transactions.amount), 0) as total").
		Joins("LEFT JOIN treatments ON treatments.id = transactions.treatment_id").
		Where("DATE(treatments.treatment_date) = ?", targetDate).
		Row().Scan(&totalAmount); err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to calculate total amount", Err: err})
		return
	}

	// Payment status counts
	type PaymentCount struct {
		Status string
		Count  int64
	}
	var paymentCounts []PaymentCount
	if err := db.Model(&model.Transaction{}).
		Select("payment_status as status, COUNT(*) as count").
		Joins("LEFT JOIN treatments ON treatments.id = transactions.treatment_id").
		Where("DATE(treatments.treatment_date) = ?", targetDate).
		Group("payment_status").
		Scan(&paymentCounts).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to calculate payment status counts", Err: err})
		return
	}

	statusSummary := model.PaymentStatusSummary{}
	for _, pc := range paymentCounts {
		switch pc.Status {
		case "paid":
			statusSummary.Paid = pc.Count
		case "partial":
			statusSummary.Partial = pc.Count
		case "unpaid":
			statusSummary.Unpaid = pc.Count
		}
	}

	// Therapist patient counts
	var therapistPatientCounts []model.TherapistPatientCount
	if err := db.Model(&model.Treatment{}).
		Select("therapists.full_name as therapist_name, COUNT(DISTINCT treatments.patient_code) as patient_count").
		Joins("LEFT JOIN therapists ON therapists.id = treatments.therapist_id").
		Where("DATE(treatments.treatment_date) = ?", targetDate).
		Group("therapists.id, therapists.full_name").
		Order("therapists.full_name").
		Scan(&therapistPatientCounts).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to calculate therapist patient counts", Err: err})
		return
	}

	summary := model.TransactionSummary{
		TotalAmount:            totalAmount,
		PaymentStatusCounts:    statusSummary,
		TherapistPatientCounts: therapistPatientCounts,
	}

	responseData := model.ListTransactionsResponseData{
		Transactions: transactions,
		Summary:      summary,
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Transactions retrieved", Data: responseData})
}

// GetTransactionInfo godoc
// @Summary      Get transaction information
// @Description  Retrieve a transaction record by ID
// @Tags         Transaction
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Transaction ID"
// @Success      200 {object} util.APIResponse{data=model.Transaction} "Transaction retrieved"
// @Failure      400 {object} util.APIResponse "Invalid ID or transaction not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /transaction/{id} [get]
func GetTransactionInfo(c *gin.Context) {
	id, ok := getTransactionIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var transaction model.Transaction
	if err := db.First(&transaction, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Transaction not found", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Transaction retrieved", Data: transaction})
}

// UpdateTransaction godoc
// @Summary      Update a transaction
// @Description  Update transaction information by ID. Only provided fields will be updated.
// @Tags         Transaction
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Transaction ID"
// @Param        request body updateTransactionRequest true "Updated transaction information"
// @Success      200 {object} util.APIResponse{data=model.Transaction} "Transaction updated"
// @Failure      400 {object} util.APIResponse "Invalid request or transaction not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /transaction/{id} [patch]
func UpdateTransaction(c *gin.Context) {
	id, ok := getTransactionIDParam(c)
	if !ok {
		return
	}

	var req updateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var transaction model.Transaction
	if err := db.First(&transaction, id).Error; err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Transaction not found", Err: err})
		return
	}

	updates := map[string]interface{}{}

	if req.Amount != nil {
		if *req.Amount < 0 {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: amount must be >= 0", Err: fmt.Errorf("invalid amount")})
			return
		}
		updates["amount"] = *req.Amount
	}

	if req.Remarks != nil {
		updates["remarks"] = *req.Remarks
	}

	if req.PaymentMethod != nil {
		updates["payment_method"] = *req.PaymentMethod
	}

	if req.PaymentStatus != nil {
		// Validate payment status
		validStatuses := map[string]bool{
			"unpaid":  true,
			"paid":    true,
			"partial": true,
		}
		if !validStatuses[*req.PaymentStatus] {
			util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: payment_status must be 'unpaid', 'paid', or 'partial'", Err: fmt.Errorf("invalid payment_status")})
			return
		}
		updates["payment_status"] = *req.PaymentStatus
	}

	if len(updates) == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "No fields to update", Err: fmt.Errorf("empty update payload")})
		return
	}

	if err := db.Model(&transaction).Updates(updates).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update transaction", Err: err})
		return
	}

	if err := db.First(&transaction, transaction.ID).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to reload transaction", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Transaction updated", Data: transaction})
}
