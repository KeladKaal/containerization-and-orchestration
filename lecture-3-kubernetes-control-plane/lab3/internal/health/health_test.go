package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name       string
		healthFail string
		wantStatus int
		wantBody   string
	}{
		{name: "healthy by default", wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "healthy with explicit false", healthFail: "false", wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "unhealthy", healthFail: "true", wantStatus: http.StatusServiceUnavailable, wantBody: "unhealthy\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HEALTH_FAIL", tt.healthFail)
			recorder := httptest.NewRecorder()
			Handler(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if recorder.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}
