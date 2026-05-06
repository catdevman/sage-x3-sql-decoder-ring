package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// tableResponse is the JSON shape returned for a single table lookup.
type tableResponse struct {
	Abbreviation string `json:"abbreviation"`
	FullName     string `json:"fullName"`
	Module       string `json:"module,omitempty"`
	DocURL       string `json:"docURL"`
}

// decodeRequest is the JSON body expected by POST /decode.
type decodeRequest struct {
	SQL string `json:"sql"`
}

// decodeResponse is the JSON shape returned by POST /decode.
type decodeResponse struct {
	Original string          `json:"original"`
	Decoded  string          `json:"decoded"`
	Tables   []tableResponse `json:"tables"`
}

func toTableResponse(t tableInfo) tableResponse {
	return tableResponse{
		Abbreviation: t.abbreviation,
		FullName:     t.fullName,
		Module:       t.module,
		DocURL:       docBaseURL,
	}
}

// newServer returns an http.Handler wired up with all routes.
//
// Routes:
//
//	GET  /tables          — list every known table
//	GET  /tables/{abbr}   — look up a single abbreviation
//	POST /decode          — decode a SQL query, return structured results
func newServer(tables map[string]tableInfo) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /tables", handleListTables(tables))
	mux.HandleFunc("GET /tables/{abbr}", handleGetTable(tables))
	mux.HandleFunc("POST /decode", handleDecode(tables))

	return mux
}

func handleListTables(tables map[string]tableInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := make([]tableResponse, 0, len(tables))
		for _, t := range tables {
			list = append(list, toTableResponse(t))
		}
		writeJSON(w, http.StatusOK, list)
	}
}

func handleGetTable(tables map[string]tableInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		abbr := strings.ToUpper(r.PathValue("abbr"))
		t, ok := tables[abbr]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "unknown abbreviation: " + abbr,
			})
			return
		}
		writeJSON(w, http.StatusOK, toTableResponse(t))
	}
}

func handleDecode(tables map[string]tableInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req decodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body: " + err.Error(),
			})
			return
		}
		if strings.TrimSpace(req.SQL) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "sql field is required",
			})
			return
		}

		decoded, found := decodeSQL(req.SQL, tables)

		resp := decodeResponse{
			Original: req.SQL,
			Decoded:  decoded,
			Tables:   make([]tableResponse, len(found)),
		}
		for i, t := range found {
			resp.Tables[i] = toTableResponse(t)
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
