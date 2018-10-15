package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFeatureFlag(t *testing.T) {
	var (
		flagName = "test"
	)

	flag := getFeatureFlag(flagName)
	if !flag {
		t.Errorf("Feature flag test on launch darkly failed to return true")
	}
}

func TestPingHandler(t *testing.T) {
	var (
		expected string
	)

	req, err := http.NewRequest("GET", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(pingHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected = "pong!\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
func TestHealthHandler(t *testing.T) {
	var (
		expected string
	)

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)

	handler.ServeHTTP(r, req)

	if status := r.Code; status != http.StatusServiceUnavailable {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusServiceUnavailable)
	}

	// Check the response body is what we expect.
	expected = "{\"status\":\"ok\"}\n"
	if r.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			r.Body.String(), expected)
	}

}
