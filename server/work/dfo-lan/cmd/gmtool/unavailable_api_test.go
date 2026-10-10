package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnavailableManagementRouteIsAuthenticatedAndCannotSucceed(t *testing.T) {
	s := &server{token: "test-token"}
	h := s.auth(s.handleUnavailableAPI)
	request := httptest.NewRequest("POST", "/api/backup/restore", nil)
	response := httptest.NewRecorder()
	h(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatal("unsupported management route bypasses auth")
	}
	request.Header.Set("X-GM-Token", "test-token")
	response = httptest.NewRecorder()
	h(response, request)
	if response.Code != http.StatusNotImplemented {
		t.Fatalf("status %d", response.Code)
	}
	var body struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.OK || body.Error == "" {
		t.Fatal("unavailable operation reported success")
	}
}
