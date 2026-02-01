package endpoint

import (
	"bytes"
	"net/http"
	"net/http/httptest"
)

// requestParams groups HTTP request parameters to reduce function arguments
type requestParams struct {
	method  string
	path    string
	body    []byte
	headers map[string]string
}

// doRequest executes an HTTP request with the given parameters and returns the response recorder
func doRequest(r http.Handler, params requestParams) (*httptest.ResponseRecorder, error) {
	req, err := http.NewRequest(params.method, params.path, bytes.NewBuffer(params.body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range params.headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr, nil
}
