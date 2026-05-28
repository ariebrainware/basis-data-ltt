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

	itemID := createTestItem(t, r)
	assertItemListSuccess(t, r)
	assertItemFetchSuccess(t, r, itemID)
	assertItemUpdateSuccess(t, r, itemID)
	assertItemDeleteSuccess(t, r, itemID)
	assertDeletedItemReturnsBadRequest(t, r, itemID)
}

func createTestItem(t *testing.T, r *gin.Engine) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/item", bytes.NewReader([]byte(`{"name":"Bandage","quantity":10,"price":25000}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create item status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("create response data missing or invalid: %#v", response["data"])
	}

	return fmt.Sprintf("%.0f", data["ID"].(float64))
}

func assertItemListSuccess(t *testing.T, r *gin.Engine) {
	t.Helper()

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/item", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list items status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func assertItemFetchSuccess(t *testing.T, r *gin.Engine, itemID string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/item/"+itemID, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("get item status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func assertItemUpdateSuccess(t *testing.T, r *gin.Engine, itemID string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/item/"+itemID, bytes.NewReader([]byte(`{"quantity":20}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update item status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal update response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("update response data missing or invalid: %#v", response["data"])
	}

	quantityValue, ok := data["quantity"].(float64)
	if !ok {
		t.Fatalf("update response quantity missing or invalid: %#v", data["quantity"])
	}
	if got := int(quantityValue); got != 20 {
		t.Fatalf("updated quantity = %d, want %d", got, 20)
	}
}

func assertItemDeleteSuccess(t *testing.T, r *gin.Engine, itemID string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/item/"+itemID, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete item status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func assertDeletedItemReturnsBadRequest(t *testing.T, r *gin.Engine, itemID string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/item/"+itemID, nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("get deleted item status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}
