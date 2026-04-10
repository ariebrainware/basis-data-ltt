package endpoint

import (
	"fmt"
	"strconv"

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
// @Success      200 {object} util.APIResponse{data=[]model.Transaction} "Transactions retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /transaction [get]
func ListTransactions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var transactions []model.Transaction
	if err := db.Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve transactions", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Transactions retrieved", Data: transactions})
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
