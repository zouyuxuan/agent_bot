package app

import (
	"net/http"
	"strings"
)

func (s *Server) handleDatasetsExportSkills(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	payload, filename, _, err := s.service.ExportTrainingSamplesAsSkills(botID)
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
