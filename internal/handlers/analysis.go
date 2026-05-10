package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(chi.URLParam(r, "ticker"))
	if ticker == "" {
		writeError(w, http.StatusBadRequest, "ticker required")
		return
	}
	var asOf *time.Time
	if asOfStr := r.URL.Query().Get("as_of"); asOfStr != "" {
		if t, err := time.Parse("2006-01-02", asOfStr); err == nil {
			asOf = &t
		} else {
			writeError(w, http.StatusBadRequest, "as_of must be YYYY-MM-DD")
			return
		}
	}
	res, err := s.Analysis.Analyze(ticker, asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
