package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransactionModel_CRUD(t *testing.T) {
	db := setupTestDB(t, "transaction", &Transaction{})

	tx := Transaction{
		TreatmentID:    1,
		TherapistID:    2,
		Amount:         250000,
		Remarks:        "Standard Session",
		PaymentMethod:  "cash",
		PaymentStatus:  "paid",
		AttachmentPath: "uploads/attachments/172468112_receipt.pdf",
		Items: []TransactionItem{
			{ItemID: 1, Quantity: 2, Price: 50000},
		},
	}

	// Create
	err := db.Create(&tx).Error
	assert.NoError(t, err)
	assert.NotZero(t, tx.ID)

	// Read
	var found Transaction
	err = db.First(&found, tx.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, tx.TreatmentID, found.TreatmentID)
	assert.Equal(t, tx.TherapistID, found.TherapistID)
	assert.Equal(t, int64(250000), found.Amount)
	assert.Equal(t, "Standard Session", found.Remarks)
	assert.Equal(t, "cash", found.PaymentMethod)
	assert.Equal(t, "paid", found.PaymentStatus)
	assert.Equal(t, "uploads/attachments/172468112_receipt.pdf", found.AttachmentPath)
	assert.Len(t, found.Items, 1)
	assert.Equal(t, uint(1), found.Items[0].ItemID)

	// Update
	newPath := "uploads/attachments/updated_receipt.png"
	err = db.Model(&found).Update("attachment_path", newPath).Error
	assert.NoError(t, err)

	var updated Transaction
	err = db.First(&updated, tx.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, newPath, updated.AttachmentPath)

	// JSON Marshal & Unmarshal
	data, err := json.Marshal(updated)
	assert.NoError(t, err)
	assert.Contains(t, string(data), newPath)

	// Delete
	err = db.Delete(&updated).Error
	assert.NoError(t, err)

	var deleted Transaction
	err = db.First(&deleted, tx.ID).Error
	assert.Error(t, err)
}
