package app

import (
	"net/http"
	"strings"
)

func (s *Server) handleDatasetsExportSkills(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var input struct {
		MemorySummary string `json:"memorySummary"`
	}
	if err := decodeOptionalJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Backward compatibility for GET /export_skills?memorySummary=...
	if strings.TrimSpace(input.MemorySummary) == "" {
		input.MemorySummary = strings.TrimSpace(r.URL.Query().Get("memorySummary"))
	}

	payload, filename, _, err := s.service.ExportTrainingSamplesAsSkills(botID, input.MemorySummary)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no training samples") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		handleStoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}
