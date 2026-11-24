package endpoint

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	employeeAttachmentDir      = "storage/employee_attachments"
	employeeAttachmentMaxBytes = 5 * 1024 * 1024
)

var unsafeFileNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

var employeeAttachmentContentTypes = map[string]string{
	".pdf":  "application/pdf",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".heic": "image/heic",
}

// loadEmployeeForAttachment parses :id and loads the employee, writing an error response on failure.
func loadEmployeeForAttachment(c *gin.Context, db *gorm.DB) (*model.Employee, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid employee ID", Err: fmt.Errorf("invalid employee id %q", c.Param("id"))})
		return nil, false
	}

	var employee model.Employee
	if err := db.First(&employee, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "Employee not found", Err: err})
		} else {
			util.CallServerError(c, util.APIErrorParams{Msg: "Database error", Err: err})
		}
		return nil, false
	}
	return &employee, true
}

// loadEmployeeAttachment loads the attachment identified by :attachmentId belonging to the given employee.
func loadEmployeeAttachment(c *gin.Context, db *gorm.DB, employeeID uint) (*model.EmployeeAttachment, bool) {
	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 64)
	if err != nil || attachmentID == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid attachment ID", Err: fmt.Errorf("invalid attachment id %q", c.Param("attachmentId"))})
		return nil, false
	}

	var attachment model.EmployeeAttachment
	if err := db.Where("id = ? AND employee_id = ?", attachmentID, employeeID).First(&attachment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "Attachment not found", Err: err})
		} else {
			util.CallServerError(c, util.APIErrorParams{Msg: "Database error", Err: err})
		}
		return nil, false
	}
	return &attachment, true
}

func employeeAttachmentDB(c *gin.Context) (*gorm.DB, bool) {
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return nil, false
	}
	return db, true
}

func employeeAttachmentFolder(employeeID uint) string {
	return filepath.Join(employeeAttachmentDir, strconv.FormatUint(uint64(employeeID), 10))
}

// sanitizeUploadFileName keeps only the base name with safe characters so it can be stored on disk.
func sanitizeUploadFileName(raw string) string {
	base := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	clean := strings.Trim(unsafeFileNameChars.ReplaceAllString(base, "_"), "._")
	if clean == "" {
		return "file"
	}
	return clean
}

// ListEmployeeAttachments godoc
// @Summary      List employee attachments
// @Description  List documents (contract, ID card, etc.) uploaded for an employee
// @Tags         Employee
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "Employee ID"
// @Success      200 {object} util.APIResponse{data=[]model.EmployeeAttachment} "Attachments retrieved"
// @Failure      400 {object} util.APIResponse "Invalid employee ID"
// @Failure      404 {object} util.APIResponse "Employee not found"
// @Router       /employee/{id}/attachment [get]
func ListEmployeeAttachments(c *gin.Context) {
	db, ok := employeeAttachmentDB(c)
	if !ok {
		return
	}
	employee, ok := loadEmployeeForAttachment(c, db)
	if !ok {
		return
	}

	attachments := []model.EmployeeAttachment{}
	if err := db.Where("employee_id = ?", employee.ID).Order("created_at DESC").Find(&attachments).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve attachments", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Attachments retrieved", Data: attachments})
}

// UploadEmployeeAttachment godoc
// @Summary      Upload an employee attachment
// @Description  Upload a document for an employee (pdf, jpeg, png, heic up to 5MB)
// @Tags         Employee
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "Employee ID"
// @Param        file formData file true "Attachment file (pdf, jpeg, png, heic up to 5MB)"
// @Param        category formData string false "contract, identity or other (default other)"
// @Success      200 {object} util.APIResponse{data=model.EmployeeAttachment} "Attachment uploaded"
// @Failure      400 {object} util.APIResponse "Invalid request, file type or size"
// @Failure      404 {object} util.APIResponse "Employee not found"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /employee/{id}/attachment [post]
func UploadEmployeeAttachment(c *gin.Context) {
	db, ok := employeeAttachmentDB(c)
	if !ok {
		return
	}
	employee, ok := loadEmployeeForAttachment(c, db)
	if !ok {
		return
	}

	// Allow some headroom over the file limit for multipart overhead
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, employeeAttachmentMaxBytes+512*1024)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || strings.Contains(err.Error(), "too large") {
			util.CallUserError(c, util.APIErrorParams{Msg: "File size exceeds the 5MB limit", Err: err})
			return
		}
		util.CallUserError(c, util.APIErrorParams{Msg: "No file was uploaded", Err: err})
		return
	}
	defer file.Close()

	if header.Size > employeeAttachmentMaxBytes {
		util.CallUserError(c, util.APIErrorParams{Msg: "File size exceeds the 5MB limit", Err: fmt.Errorf("file size %d exceeds limit", header.Size)})
		return
	}

	category := strings.ToLower(strings.TrimSpace(c.PostForm("category")))
	if category == "" {
		category = model.EmployeeAttachmentCategoryOther
	}
	if !model.EmployeeAttachmentCategories[category] {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid category. Allowed: contract, identity, other", Err: fmt.Errorf("invalid category %q", category)})
		return
	}

	buf := make([]byte, 512)
	n, _ := io.ReadFull(file, buf)
	if n == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Empty files are not allowed", Err: fmt.Errorf("uploaded file is empty")})
		return
	}
	if !isAllowedTransactionAttachmentType(header.Filename, buf[:n]) {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid file type. Allowed types: pdf, jpeg, png, heic", Err: fmt.Errorf("unsupported file %s", header.Filename)})
		return
	}

	dir := employeeAttachmentFolder(employee.ID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to create upload directory", Err: err})
		return
	}

	cleanName := sanitizeUploadFileName(header.Filename)
	storedName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), cleanName)
	storedPath := filepath.Join(dir, storedName)
	if err := c.SaveUploadedFile(header, storedPath); err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to save file", Err: err})
		return
	}

	attachment := model.EmployeeAttachment{
		EmployeeID:  employee.ID,
		Category:    category,
		FileName:    filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/")),
		StoredName:  storedName,
		ContentType: employeeAttachmentContentTypes[strings.ToLower(filepath.Ext(cleanName))],
		Size:        header.Size,
	}
	if err := db.Create(&attachment).Error; err != nil {
		_ = os.Remove(storedPath)
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to save attachment", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Attachment uploaded", Data: attachment})
}

// DownloadEmployeeAttachment godoc
// @Summary      Download an employee attachment
// @Description  Download a document uploaded for an employee (Admin only)
// @Tags         Employee
// @Produce      octet-stream
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "Employee ID"
// @Param        attachmentId path int true "Attachment ID"
// @Success      200 {file} file "Attachment binary content"
// @Failure      404 {object} util.APIResponse "Attachment not found"
// @Router       /employee/{id}/attachment/{attachmentId} [get]
func DownloadEmployeeAttachment(c *gin.Context) {
	db, ok := employeeAttachmentDB(c)
	if !ok {
		return
	}
	employee, ok := loadEmployeeForAttachment(c, db)
	if !ok {
		return
	}
	attachment, ok := loadEmployeeAttachment(c, db, employee.ID)
	if !ok {
		return
	}

	path, err := resolveAttachmentPath(employeeAttachmentFolder(employee.ID), attachment.StoredName)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Invalid attachment path", Err: err})
		return
	}
	if fi, statErr := os.Stat(path); statErr != nil || fi.IsDir() {
		util.CallErrorNotFound(c, util.APIErrorParams{Msg: "Attachment file not found", Err: fmt.Errorf("missing file for attachment %d", attachment.ID)})
		return
	}

	if attachment.ContentType != "" {
		c.Header("Content-Type", attachment.ContentType)
	}
	c.FileAttachment(path, attachment.FileName)
}

// DeleteEmployeeAttachment godoc
// @Summary      Delete an employee attachment
// @Description  Permanently delete a document uploaded for an employee
// @Tags         Employee
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "Employee ID"
// @Param        attachmentId path int true "Attachment ID"
// @Success      200 {object} util.APIResponse "Attachment deleted"
// @Failure      404 {object} util.APIResponse "Attachment not found"
// @Router       /employee/{id}/attachment/{attachmentId} [delete]
func DeleteEmployeeAttachment(c *gin.Context) {
	db, ok := employeeAttachmentDB(c)
	if !ok {
		return
	}
	employee, ok := loadEmployeeForAttachment(c, db)
	if !ok {
		return
	}
	attachment, ok := loadEmployeeAttachment(c, db, employee.ID)
	if !ok {
		return
	}

	if err := db.Unscoped().Delete(attachment).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete attachment", Err: err})
		return
	}

	if path, err := resolveAttachmentPath(employeeAttachmentFolder(employee.ID), attachment.StoredName); err == nil {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			// Record is gone already; leave the orphaned file rather than failing the request.
			_ = c.Error(rmErr)
		}
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Attachment deleted", Data: nil})
}
