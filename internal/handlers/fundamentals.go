package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleFundamentals(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(chi.URLParam(r, "ticker"))
	if ticker == "" {
		writeError(w, http.StatusBadRequest, "ticker required")
		return
	}
	f, err := s.Fundamentals.Get(ticker)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}
