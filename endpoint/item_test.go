package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupItemTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	r, _ := setupEndpointTest(t)
	r.POST("/item", CreateItem)
	r.GET("/item", ListItems)
	r.GET("/item/:id", GetItemInfo)
	r.PATCH("/item/:id", UpdateItem)
	r.DELETE("/item/:id", DeleteItem)
	return r
}

func TestItemCRUDFlow(t *testing.T) {
	r := setupItemTestRouter(t)

	createRecorder := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/item", bytes.NewReader([]byte(`{"name":"Bandage","quantity":10,"price":25000}`)))
	createReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create item status = %d, want %d, body=%s", createRecorder.Code, http.StatusOK, createRecorder.Body.String())
	}

	var createResp map[string]interface{}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	data, ok := createResp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("create response data missing or invalid: %#v", createResp["data"])
	}
	itemID := fmt.Sprintf("%.0f", data["ID"].(float64))

	listRecorder := httptest.NewRecorder()
	r.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/item", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list items status = %d, want %d, body=%s", listRecorder.Code, http.StatusOK, listRecorder.Body.String())
	}

	getRecorder := httptest.NewRecorder()
	r.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/item/"+itemID, nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get item status = %d, want %d, body=%s", getRecorder.Code, http.StatusOK, getRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPatch, "/item/"+itemID, bytes.NewReader([]byte(`{"quantity":20}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(updateRecorder, updateReq)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update item status = %d, want %d, body=%s", updateRecorder.Code, http.StatusOK, updateRecorder.Body.String())
	}
	var updateResp map[string]interface{}
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("unmarshal update response: %v", err)
	}
	updateData, ok := updateResp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("update response data missing or invalid: %#v", updateResp["data"])
	}
	quantityValue, ok := updateData["quantity"].(float64)
	if !ok {
		t.Fatalf("update response quantity missing or invalid: %#v", updateData["quantity"])
	}
	if got := int(quantityValue); got != 20 {
		t.Fatalf("updated quantity = %d, want %d", got, 20)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/item/"+itemID, nil)
	r.ServeHTTP(deleteRecorder, deleteReq)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete item status = %d, want %d, body=%s", deleteRecorder.Code, http.StatusOK, deleteRecorder.Body.String())
	}

	getDeletedRecorder := httptest.NewRecorder()
	r.ServeHTTP(getDeletedRecorder, httptest.NewRequest(http.MethodGet, "/item/"+itemID, nil))
	if getDeletedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("get deleted item status = %d, want %d, body=%s", getDeletedRecorder.Code, http.StatusBadRequest, getDeletedRecorder.Body.String())
	}
}
