package model

import "gorm.io/gorm"

// Expense represents an expense record
// @Description Expense entity information
type Expense struct {
	gorm.Model
	ExpenseDate   string `json:"expense_date" gorm:"column:expense_date;size:50;not null;index" example:"2025-01-15"`
	Category      string `json:"category" gorm:"column:category;size:100;not null;index" example:"Operational"`
	Amount        int64  `json:"amount" gorm:"column:amount;not null" example:"150000"`
	Description   string `json:"description" gorm:"column:description;type:text;not null" example:"Electricity bill for clinic"`
	PaymentMethod string `json:"payment_method" gorm:"column:payment_method;size:50;not null" example:"bank_transfer"`
	ReceiptURL    string `json:"receipt_url,omitempty" gorm:"column:receipt_url;type:text" example:"https://example.com/receipts/rec-123.jpg"`
	Notes         string `json:"notes,omitempty" gorm:"column:notes;type:text" example:"Paid on time"`
}

// CreateExpenseRequest represents the expense creation payload
// @Description Expense creation request payload
type CreateExpenseRequest struct {
	ExpenseDate   string `json:"expense_date" binding:"required" example:"2025-01-15"`
	Category      string `json:"category" binding:"required" example:"Operational"`
	Amount        int64  `json:"amount" binding:"required,gt=0" example:"150000"`
	Description   string `json:"description" binding:"required" example:"Electricity bill for clinic"`
	PaymentMethod string `json:"payment_method" binding:"required" example:"bank_transfer"`
	ReceiptURL    string `json:"receipt_url,omitempty" example:"https://example.com/receipts/rec-123.jpg"`
	Notes         string `json:"notes,omitempty" example:"Paid on time"`
}

// UpdateExpenseRequest represents the expense update payload
// @Description Expense update request payload
type UpdateExpenseRequest struct {
	ExpenseDate   *string `json:"expense_date" example:"2025-01-15"`
	Category      *string `json:"category" example:"Operational"`
	Amount        *int64  `json:"amount" example:"150000"`
	Description   *string `json:"description" example:"Electricity bill for clinic"`
	PaymentMethod *string `json:"payment_method" example:"bank_transfer"`
	ReceiptURL    *string `json:"receipt_url" example:"https://example.com/receipts/rec-123.jpg"`
	Notes         *string `json:"notes" example:"Paid on time"`
}

// CategoryExpenseBreakdown represents the sum of expenses for a specific category
// @Description Expense category aggregation
type CategoryExpenseBreakdown struct {
	Category    string `json:"category" example:"Operational"`
	TotalAmount int64  `json:"total_amount" example:"4500000"`
	Count       int64  `json:"count" example:"12"`
}

// ExpenseSummary represents aggregated expense statistics
// @Description Expense summary statistics
type ExpenseSummary struct {
	TotalAmount       int64                      `json:"total_amount" example:"4500000"`
	TotalCount        int64                      `json:"total_count" example:"12"`
	CategoryBreakdown []CategoryExpenseBreakdown `json:"category_breakdown"`
}

// ListExpensesResponseData represents paginated expenses along with high-level summary
// @Description Paginated expense list with summary
type ListExpensesResponseData struct {
	Expenses []Expense      `json:"expenses"`
	Summary  ExpenseSummary `json:"summary"`
}
