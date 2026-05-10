package handlers

import "net/http"

func (s *Server) handleMacroSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := s.MacroService.Snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
