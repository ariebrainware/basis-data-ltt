package endpoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type transactionItemRequest struct {
	ItemID   uint `json:"item_id"`
	Quantity int  `json:"quantity"`
}

type updateTransactionRequest struct {
	Amount        *int64                   `json:"amount"`
	Remarks       *string                  `json:"remarks"`
	PaymentMethod *string                  `json:"payment_method"`
	PaymentStatus *string                  `json:"payment_status"`
	Items         []transactionItemRequest `json:"items"`
}

type transactionUserError struct {
	msg string
}

func (e *transactionUserError) Error() string {
	return e.msg
}

func isValidTransactionPaymentStatus(status string) bool {
	validStatuses := map[string]bool{
		"unpaid":  true,
		"partial": true,
		"paid":    true,
	}

	return validStatuses[status]
}

func isValidTransactionPaymentMethod(method string) bool {
	validMethods := map[string]bool{
		"cash":             true,
		"transfer_or_qris": true,
		"debit":            true,
	}

	return validMethods[method]
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

type transactionDateScope struct {
	startDate string
	endDate   string
}

func normalizeTransactionDate(raw string) (string, error) {
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

func getTransactionDateScope(c *gin.Context) (transactionDateScope, error) {
	treatmentDate := c.Query("treatment_date")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	hasTreatmentDate := treatmentDate != ""
	hasRange := startDate != "" || endDate != ""

	if hasTreatmentDate && hasRange {
		return transactionDateScope{}, fmt.Errorf("use either treatment_date or start_date/end_date")
	}

	if hasRange {
		if startDate == "" || endDate == "" {
			return transactionDateScope{}, fmt.Errorf("both start_date and end_date are required")
		}

		normalizedStart, err := normalizeTransactionDate(startDate)
		if err != nil {
			return transactionDateScope{}, err
		}

		normalizedEnd, err := normalizeTransactionDate(endDate)
		if err != nil {
			return transactionDateScope{}, err
		}

		if normalizedStart > normalizedEnd {
			return transactionDateScope{}, fmt.Errorf("start_date must be before or equal to end_date")
		}

		return transactionDateScope{startDate: normalizedStart, endDate: normalizedEnd}, nil
	}

	if hasTreatmentDate {
		normalizedDate, err := normalizeTransactionDate(treatmentDate)
		if err != nil {
			return transactionDateScope{}, err
		}

		return transactionDateScope{startDate: normalizedDate, endDate: normalizedDate}, nil
	}

	return transactionDateScope{}, nil
}

func applyTransactionDateScope(query *gorm.DB, scope transactionDateScope) *gorm.DB {
	if scope.startDate == "" {
		return query
	}

	if scope.startDate == scope.endDate {
		return query.Where("DATE(treatments.treatment_date) = ?", scope.startDate)
	}

	return query.Where("DATE(treatments.treatment_date) BETWEEN ? AND ?", scope.startDate, scope.endDate)
}

func calculateTransactionAmountAndAdjustItems(tx *gorm.DB, therapistID uint, items []transactionItemRequest) (int64, error) {
	var pricing model.Pricing
	if err := tx.Model(&model.Pricing{}).Where("therapist_id = ?", therapistID).Order("id DESC").First(&pricing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, &transactionUserError{msg: "pricing not found for therapist"}
		}
		return 0, err
	}

	totalAmount := pricing.Price
	if len(items) == 0 {
		return totalAmount, nil
	}

	aggregatedQuantities := make(map[uint]int)
	for _, itemRequest := range items {
		if itemRequest.ItemID == 0 {
			return 0, &transactionUserError{msg: "item_id is required for each item"}
		}
		if itemRequest.Quantity <= 0 {
			return 0, &transactionUserError{msg: "item quantity must be greater than 0"}
		}
		aggregatedQuantities[itemRequest.ItemID] += itemRequest.Quantity
	}

	itemIDs := make([]uint, 0, len(aggregatedQuantities))
	for itemID := range aggregatedQuantities {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })

	for _, itemID := range itemIDs {
		quantity := aggregatedQuantities[itemID]

		var item model.Item
		if err := tx.Where("id = ? AND deleted_at IS NULL", itemID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, &transactionUserError{msg: fmt.Sprintf("item %d not found", itemID)}
			}
			return 0, err
		}

		if item.Quantity < quantity {
			return 0, &transactionUserError{msg: fmt.Sprintf("insufficient stock for item %d", itemID)}
		}

		newQuantity := item.Quantity - quantity
		if err := tx.Model(&item).Update("quantity", newQuantity).Error; err != nil {
			return 0, err
		}

		totalAmount += int64(quantity) * item.Price
	}

	return totalAmount, nil
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
// @Param        treatment_date query string false "Filter by treatment date (YYYY-MM-DD). Also accepts RFC3339 datetime."
// @Param        start_date query string false "Start date for a date range filter (YYYY-MM-DD)."
// @Param        end_date query string false "End date for a date range filter (YYYY-MM-DD)."
// @Success      200 {object} util.APIResponse{data=model.ListTransactionsResponseData} "Transactions retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /transaction [get]
func ListTransactions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	dateScope, err := getTransactionDateScope(c)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid date filter. Use treatment_date or start_date/end_date with YYYY-MM-DD or RFC3339 values", Err: err})
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

	txQuery = applyTransactionDateScope(txQuery, dateScope)

	if err := txQuery.
		Order("transactions.id DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve transactions", Err: err})
		return
	}

	// Calculate summary for the same filter scope as transaction list.
	summaryScope := dateScope

	// Total amount for the target date
	var totalAmount int64
	if err := applyTransactionDateScope(db.Model(&model.Transaction{}), summaryScope).
		Select("COALESCE(SUM(transactions.amount), 0) as total").
		Joins("LEFT JOIN treatments ON treatments.id = transactions.treatment_id").
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
	if err := applyTransactionDateScope(db.Model(&model.Transaction{}), summaryScope).
		Select("payment_status as status, COUNT(*) as count").
		Joins("LEFT JOIN treatments ON treatments.id = transactions.treatment_id").
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
	if err := applyTransactionDateScope(db.Model(&model.Treatment{}), summaryScope).
		Select("therapists.full_name as therapist_name, COUNT(DISTINCT treatments.patient_code) as patient_count").
		Joins("LEFT JOIN therapists ON therapists.id = treatments.therapist_id").
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

	err := db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{}

		if req.Amount != nil {
			if *req.Amount < 0 {
				return &transactionUserError{msg: "Invalid request body: amount must be >= 0"}
			}
			updates["amount"] = *req.Amount
		}

		if len(req.Items) > 0 {
			// Refund stock of currently associated items first
			for _, existingItem := range transaction.Items {
				var item model.Item
				if err := tx.Where("id = ? AND deleted_at IS NULL", existingItem.ItemID).First(&item).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}
				newQuantity := item.Quantity + existingItem.Quantity
				if err := tx.Model(&item).Update("quantity", newQuantity).Error; err != nil {
					return err
				}
			}

			computedAmount, err := calculateTransactionAmountAndAdjustItems(tx, transaction.TherapistID, req.Items)
			if err != nil {
				return err
			}
			updates["amount"] = computedAmount

			var modelItems []model.TransactionItem
			for _, itemReq := range req.Items {
				modelItems = append(modelItems, model.TransactionItem{
					ItemID:   itemReq.ItemID,
					Quantity: itemReq.Quantity,
				})
			}
			itemsJSON, err := json.Marshal(modelItems)
			if err != nil {
				return err
			}
			updates["items"] = string(itemsJSON)
		}

		if req.Remarks != nil {
			updates["remarks"] = *req.Remarks
		}

		if req.PaymentMethod != nil {
			if !isValidTransactionPaymentMethod(*req.PaymentMethod) {
				return &transactionUserError{msg: "Invalid request body: payment_method must be 'cash', 'transfer_or_qris', or 'debit'"}
			}
			updates["payment_method"] = *req.PaymentMethod
		}

		if req.PaymentStatus != nil {
			if !isValidTransactionPaymentStatus(*req.PaymentStatus) {
				return &transactionUserError{msg: "Invalid request body: payment_status must be 'paid', 'partial', or 'unpaid'"}
			}
			updates["payment_status"] = *req.PaymentStatus
		}

		if len(updates) == 0 {
			return &transactionUserError{msg: "No fields to update"}
		}

		if err := tx.Model(&transaction).Updates(updates).Error; err != nil {
			return err
		}

		return tx.First(&transaction, transaction.ID).Error
	})
	if err != nil {
		var userErr *transactionUserError
		if errors.As(err, &userErr) {
			util.CallUserError(c, util.APIErrorParams{Msg: userErr.msg, Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update transaction", Err: err})
		return
	}

	if err := db.First(&transaction, transaction.ID).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to reload transaction", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Transaction updated", Data: transaction})
}
