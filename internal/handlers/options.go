package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleOptionsBias(w http.ResponseWriter, r *http.Request) {
	ticker := strings.ToUpper(chi.URLParam(r, "ticker"))
	if ticker == "" {
		writeError(w, http.StatusBadRequest, "ticker required")
		return
	}
	priceStr := r.URL.Query().Get("price")
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		writeError(w, http.StatusBadRequest, "price query param required")
		return
	}
	bias := s.Options.AnalyzeBias(ticker, price)
	writeJSON(w, http.StatusOK, bias)
}
