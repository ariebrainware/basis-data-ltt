package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var samplePDF = []byte("%PDF-1.4 employee contract")

func setupEmployeeAttachmentTest(t *testing.T) (*gin.Engine, *gorm.DB, model.Employee) {
	t.Helper()
	r, db := setupEndpointTest(t)
	db.Where("1 = 1").Delete(&model.EmployeeAttachment{})

	r.GET("/employee/:id/attachment", ListEmployeeAttachments)
	r.POST("/employee/:id/attachment", UploadEmployeeAttachment)
	r.GET("/employee/:id/attachment/:attachmentId", DownloadEmployeeAttachment)
	r.DELETE("/employee/:id/attachment/:attachmentId", DeleteEmployeeAttachment)

	employee := model.Employee{
		NIK: "3201000000000001", FullName: "Jane Doe", Gender: "Female", Address: "Jl. Mawar",
		Religion: "Islam", PhoneNumber: "081200000000", Email: "jane@example.com",
		JoinedDate: "2026-01-01", Position: "Staff", BaseSalary: 1, LunchMoney: 1,
	}
	require.NoError(t, db.Create(&employee).Error)
	t.Cleanup(func() {
		_ = os.RemoveAll(employeeAttachmentFolder(employee.ID))
	})
	return r, db, employee
}

func uploadEmployeeAttachment(t *testing.T, r *gin.Engine, employeeID uint, filename, category string, content []byte) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	if category != "" {
		require.NoError(t, writer.WriteField("category", category))
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/employee/%d/attachment", employeeID), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w, resp
}

func serveEmployeeAttachment(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUploadEmployeeAttachment_SavesRecordAndFile(t *testing.T) {
	r, db, employee := setupEmployeeAttachmentTest(t)

	w, resp := uploadEmployeeAttachment(t, r, employee.ID, "Kontrak Kerja 2026.pdf", "contract", samplePDF)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "Kontrak Kerja 2026.pdf", data["file_name"])
	assert.Equal(t, "contract", data["category"])
	assert.Equal(t, "application/pdf", data["content_type"])
	assert.NotContains(t, data, "stored_name")

	var saved model.EmployeeAttachment
	require.NoError(t, db.Where("employee_id = ?", employee.ID).First(&saved).Error)
	assert.Contains(t, saved.StoredName, "Kontrak_Kerja_2026.pdf")
	content, err := os.ReadFile(filepath.Join(employeeAttachmentFolder(employee.ID), saved.StoredName))
	require.NoError(t, err)
	assert.Equal(t, samplePDF, content)
}

func TestUploadEmployeeAttachment_DefaultsCategoryToOther(t *testing.T) {
	r, _, employee := setupEmployeeAttachmentTest(t)

	w, resp := uploadEmployeeAttachment(t, r, employee.ID, "ktp.pdf", "", samplePDF)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "other", resp["data"].(map[string]interface{})["category"])
}

func TestUploadEmployeeAttachment_Rejections(t *testing.T) {
	r, _, employee := setupEmployeeAttachmentTest(t)

	cases := []struct {
		name       string
		employeeID uint
		filename   string
		category   string
		content    []byte
		status     int
		msg        string
	}{
		{"unknown employee", employee.ID + 999, "a.pdf", "", samplePDF, http.StatusNotFound, "Employee not found"},
		{"no file", employee.ID, "", "", nil, http.StatusBadRequest, "No file was uploaded"},
		{"empty file", employee.ID, "a.pdf", "", []byte{}, http.StatusBadRequest, "Empty files are not allowed"},
		{"bad type", employee.ID, "a.exe", "", []byte("MZ\x90\x00"), http.StatusBadRequest, "Allowed types"},
		{"fake pdf", employee.ID, "a.pdf", "", []byte("not a pdf"), http.StatusBadRequest, "Allowed types"},
		{"bad category", employee.ID, "a.pdf", "salary", samplePDF, http.StatusBadRequest, "Invalid category"},
		{"too large", employee.ID, "a.pdf", "", append([]byte("%PDF-"), make([]byte, employeeAttachmentMaxBytes)...), http.StatusBadRequest, "5MB limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, resp := uploadEmployeeAttachment(t, r, tc.employeeID, tc.filename, tc.category, tc.content)
			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, resp["msg"], tc.msg)
		})
	}
}

func TestEmployeeAttachment_ListDownloadDelete(t *testing.T) {
	r, db, employee := setupEmployeeAttachmentTest(t)

	w, resp := uploadEmployeeAttachment(t, r, employee.ID, "contract.pdf", "contract", samplePDF)
	require.Equal(t, http.StatusOK, w.Code)
	id := uint(resp["data"].(map[string]interface{})["ID"].(float64))

	// List
	w = serveEmployeeAttachment(r, http.MethodGet, fmt.Sprintf("/employee/%d/attachment", employee.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var listResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	assert.Len(t, listResp["data"], 1)

	// Download
	w = serveEmployeeAttachment(r, http.MethodGet, fmt.Sprintf("/employee/%d/attachment/%d", employee.ID, id))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, samplePDF, w.Body.Bytes())
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "contract.pdf")

	// Another employee cannot reach this attachment
	other := employee
	other.ID = 0
	other.NIK = "3201000000000002"
	require.NoError(t, db.Create(&other).Error)
	w = serveEmployeeAttachment(r, http.MethodGet, fmt.Sprintf("/employee/%d/attachment/%d", other.ID, id))
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = serveEmployeeAttachment(r, http.MethodDelete, fmt.Sprintf("/employee/%d/attachment/%d", other.ID, id))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Delete removes the record and the file
	var saved model.EmployeeAttachment
	require.NoError(t, db.First(&saved, id).Error)
	storedPath := filepath.Join(employeeAttachmentFolder(employee.ID), saved.StoredName)

	w = serveEmployeeAttachment(r, http.MethodDelete, fmt.Sprintf("/employee/%d/attachment/%d", employee.ID, id))
	require.Equal(t, http.StatusOK, w.Code)
	var count int64
	db.Unscoped().Model(&model.EmployeeAttachment{}).Where("id = ?", id).Count(&count)
	assert.Zero(t, count)
	_, err := os.Stat(storedPath)
	assert.True(t, os.IsNotExist(err))

	w = serveEmployeeAttachment(r, http.MethodGet, fmt.Sprintf("/employee/%d/attachment/%d", employee.ID, id))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSanitizeUploadFileName(t *testing.T) {
	assert.Equal(t, "passwd", sanitizeUploadFileName("../../etc/passwd"))
	assert.Equal(t, "evil.pdf", sanitizeUploadFileName(`C:\Users\x\evil.pdf`))
	assert.Equal(t, "Kontrak_Kerja.pdf", sanitizeUploadFileName("Kontrak Kerja.pdf"))
	assert.Equal(t, "file", sanitizeUploadFileName(".."))
}
