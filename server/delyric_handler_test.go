package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshsymonds/sound-stage/server"
)

func TestDelyricProxyHandler(t *testing.T) {
	t.Parallel()

	t.Run("proxies POST to delyric worker", func(t *testing.T) {
		t.Parallel()
		worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"jobId":"abc123"}`))
		}))
		defer worker.Close()

		handler := server.DelyricProxyHandler(worker.URL, http.MethodPost, "/process")
		body := `{"songPath":"/mnt/music/sound-stage/Queen - Bohemian Rhapsody"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/delyric/process", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "abc123") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("returns 503 when worker unreachable", func(t *testing.T) {
		t.Parallel()
		handler := server.DelyricProxyHandler("http://127.0.0.1:1", http.MethodPost, "/process")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/delyric/process", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("returns 503 when not configured", func(t *testing.T) {
		t.Parallel()
		handler := server.DelyricProxyHandler("", http.MethodPost, "/process")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/delyric/process", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})
}
