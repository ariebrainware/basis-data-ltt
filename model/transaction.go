package model

import "gorm.io/gorm"

// TransactionItem represents an item consumed by a transaction.
// @Description Transaction item detail information
type TransactionItem struct {
	ItemID   uint `json:"item_id" example:"1"`
	Quantity int  `json:"quantity" example:"2"`
}

// Transaction represents a payment transaction for a treatment.
// @Description Transaction information for a treatment
type Transaction struct {
	gorm.Model
	TreatmentID   uint   `json:"treatment_id" gorm:"not null;index" example:"1"`
	TherapistID   uint   `json:"therapist_id" gorm:"not null;index" example:"1"`
	Amount        int64  `json:"amount" gorm:"not null" example:"50000"`
	Remarks       string `json:"remarks" example:"Urgent handling fee"`
	PaymentMethod string `json:"payment_method" example:"cash"`
	PaymentStatus string `json:"payment_status" gorm:"default:'unpaid'" example:"unpaid"`
	Items         []TransactionItem `json:"items,omitempty" gorm:"serializer:json;type:json"`
}

// ListTransactionResponse represents transaction list data with patient details.
// @Description Transaction list response information
type ListTransactionResponse struct {
	Transaction
	PatientName   string `json:"patient_name" gorm:"column:patient_name" example:"John Doe"`
	TreatmentDate string `json:"treatment_date" gorm:"column:treatment_date" example:"2025-01-15"`
}

// PaymentStatusSummary represents counts of transactions by payment status.
type PaymentStatusSummary struct {
	Paid    int64 `json:"paid" example:"5"`
	Partial int64 `json:"partial" example:"2"`
	Unpaid  int64 `json:"unpaid" example:"3"`
}

// TherapistPatientCount represents the count of unique patients handled by a therapist.
type TherapistPatientCount struct {
	TherapistName string `json:"therapist_name" example:"Dr. John Smith"`
	PatientCount  int64  `json:"patient_count" example:"10"`
}

// TransactionSummary represents aggregated transaction data.
type TransactionSummary struct {
	TotalAmount            int64                   `json:"total_amount" example:"1250000"`
	PaymentStatusCounts    PaymentStatusSummary    `json:"payment_status_counts"`
	TherapistPatientCounts []TherapistPatientCount `json:"therapist_patient_counts"`
}

// ListTransactionsResponseData groups transactions with summary information.
type ListTransactionsResponseData struct {
	Transactions []ListTransactionResponse `json:"transactions"`
	Summary      TransactionSummary        `json:"summary"`
}
