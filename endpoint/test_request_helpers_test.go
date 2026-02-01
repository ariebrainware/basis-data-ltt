package endpoint

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
)

type requestSpec struct {
	method       string
	registerPath string
	requestPath  string
	handler      gin.HandlerFunc
	body         interface{}
	headers      map[string]string
}

func performRequest(r *gin.Engine, spec requestSpec) (*httptest.ResponseRecorder, map[string]interface{}, error) {
	var reader *strings.Reader
	setJSONHeader := false
	switch v := spec.body.(type) {
	case nil:
		reader = strings.NewReader("")
	case string:
		reader = strings.NewReader(v)
		setJSONHeader = true
	default:
		b, _ := json.Marshal(spec.body)
		reader = strings.NewReader(string(b))
		setJSONHeader = true
	}

	req := httptest.NewRequest(spec.method, spec.requestPath, reader)
	if setJSONHeader {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range spec.headers {
		req.Header.Set(key, value)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var response map[string]interface{}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			return w, nil, err
		}
	}
	return w, response, nil
}

func doRequestWithHandler(r *gin.Engine, spec requestSpec) (*httptest.ResponseRecorder, map[string]interface{}, error) {
	switch spec.method {
	case http.MethodGet:
		r.GET(spec.registerPath, spec.handler)
	case http.MethodPost:
		r.POST(spec.registerPath, spec.handler)
	case http.MethodPatch:
		r.PATCH(spec.registerPath, spec.handler)
	case http.MethodPut:
		r.PUT(spec.registerPath, spec.handler)
	case http.MethodDelete:
		r.DELETE(spec.registerPath, spec.handler)
	default:
		r.Handle(spec.method, spec.registerPath, spec.handler)
	}
	return performRequest(r, spec)
}
