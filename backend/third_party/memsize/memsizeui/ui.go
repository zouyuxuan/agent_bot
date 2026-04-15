package memsizeui

import (
	"net/http"
)

// Handler is a minimal stub for github.com/fjl/memsize/memsizeui.Handler.
// It keeps go-ethereum's internal/debug package buildable on Go 1.23+.
type Handler struct{}

func (h *Handler) Add(_ string, _ interface{}) {}

func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte("memsize UI disabled (stubbed for Go 1.23 compatibility)\n"))
}
