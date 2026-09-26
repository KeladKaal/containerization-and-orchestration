package health

import (
	"net/http"
	"os"
	"strconv"
)

func Handler(w http.ResponseWriter, _ *http.Request) {
	fail, _ := strconv.ParseBool(os.Getenv("HEALTH_FAIL"))
	if fail {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
