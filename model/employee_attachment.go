package model

import "gorm.io/gorm"

// Employee attachment categories
const (
	EmployeeAttachmentCategoryContract = "contract"
	EmployeeAttachmentCategoryIdentity = "identity"
	EmployeeAttachmentCategoryOther    = "other"
)

// EmployeeAttachmentCategories lists the accepted attachment categories.
var EmployeeAttachmentCategories = map[string]bool{
	EmployeeAttachmentCategoryContract: true,
	EmployeeAttachmentCategoryIdentity: true,
	EmployeeAttachmentCategoryOther:    true,
}

// EmployeeAttachment is a document (contract, ID card, etc.) stored for an employee.
// @Description Employee document attachment
type EmployeeAttachment struct {
	gorm.Model
	EmployeeID  uint   `json:"employee_id" gorm:"column:employee_id;index;not null" example:"1"`
	Category    string `json:"category" gorm:"column:category;type:varchar(30);not null" example:"contract"`
	FileName    string `json:"file_name" gorm:"column:file_name;not null" example:"kontrak-2026.pdf"`
	StoredName  string `json:"-" gorm:"column:stored_name;not null"`
	ContentType string `json:"content_type" gorm:"column:content_type;type:varchar(100)" example:"application/pdf"`
	Size        int64  `json:"size" gorm:"column:size" example:"204800"`
}
