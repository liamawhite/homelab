package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardRoutes(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/saved-bookmark"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			if !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
			}
			if !strings.Contains(response.Body.String(), "<title>Dashboard</title>") {
				t.Fatal("dashboard document missing")
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/saved-bookmark", nil))
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("HEAD status = %d, body length = %d", response.Code, response.Body.Len())
	}
}

func TestManifestIsServedAsAnAsset(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/manifest.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Header().Get("Content-Type"), "application/json") || !strings.Contains(string(body), `"name": "Dashboard"`) {
		t.Fatalf("manifest response = %s", body)
	}
}
