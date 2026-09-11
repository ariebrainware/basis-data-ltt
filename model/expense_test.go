package model_test

import (
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/stretchr/testify/assert"
)

func TestExpenseModelInstantiation(t *testing.T) {
	expense := model.Expense{
		ExpenseDate:   "2025-01-15",
		Category:      "Operational",
		Amount:        150000,
		Description:   "Electricity bill for clinic",
		PaymentMethod: "bank_transfer",
		ReceiptURL:    "https://example.com/receipts/rec-123.jpg",
		Notes:         "Paid on time",
	}

	assert.Equal(t, "2025-01-15", expense.ExpenseDate)
	assert.Equal(t, "Operational", expense.Category)
	assert.Equal(t, int64(150000), expense.Amount)
	assert.Equal(t, "Electricity bill for clinic", expense.Description)
	assert.Equal(t, "bank_transfer", expense.PaymentMethod)
	assert.Equal(t, "https://example.com/receipts/rec-123.jpg", expense.ReceiptURL)
	assert.Equal(t, "Paid on time", expense.Notes)
}

func TestExpenseSummaryStructs(t *testing.T) {
	breakdown := []model.CategoryExpenseBreakdown{
		{Category: "Operational", TotalAmount: 150000, Count: 1},
		{Category: "Supplies", TotalAmount: 250000, Count: 2},
	}

	summary := model.ExpenseSummary{
		TotalAmount:       400000,
		TotalCount:        3,
		CategoryBreakdown: breakdown,
	}

	listData := model.ListExpensesResponseData{
		Expenses: []model.Expense{
			{Amount: 150000, Category: "Operational"},
			{Amount: 250000, Category: "Supplies"},
		},
		Summary: summary,
	}

	assert.Equal(t, int64(400000), listData.Summary.TotalAmount)
	assert.Equal(t, int64(3), listData.Summary.TotalCount)
	assert.Len(t, listData.Summary.CategoryBreakdown, 2)
	assert.Len(t, listData.Expenses, 2)
}
