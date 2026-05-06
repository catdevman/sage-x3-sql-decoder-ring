package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

var serverTables = map[string]tableInfo{
	"BPC": {abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	"BPS": {abbreviation: "BPS", fullName: "Suppliers", module: "Common Data"},
	"BAL": {abbreviation: "BAL", fullName: "General balance", module: "Financials"},
}

// get is a helper that fires a GET against the test server and returns the response.
func get(t *testing.T, srv http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

// post is a helper that fires a POST with a JSON body.
func post(t *testing.T, srv http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

// decodeBody unmarshals the recorder body into v.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- GET /tables ---

func TestHandleListTables_StatusOK(t *testing.T) {
	srv := newServer(serverTables)
	w := get(t, srv, "/tables")
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandleListTables_ContentType(t *testing.T) {
	srv := newServer(serverTables)
	w := get(t, srv, "/tables")
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("want application/json, got %q", ct)
	}
}

func TestHandleListTables_ReturnsAllEntries(t *testing.T) {
	srv := newServer(serverTables)
	w := get(t, srv, "/tables")

	var list []tableResponse
	decodeBody(t, w, &list)

	if len(list) != len(serverTables) {
		t.Errorf("want %d entries, got %d", len(serverTables), len(list))
	}
}

func TestHandleListTables_EntryShape(t *testing.T) {
	srv := newServer(map[string]tableInfo{
		"BPC": {abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	})
	w := get(t, srv, "/tables")

	var list []tableResponse
	decodeBody(t, w, &list)

	if len(list) != 1 {
		t.Fatalf("want 1 entry, got %d", len(list))
	}
	got := list[0]
	if got.Abbreviation != "BPC" {
		t.Errorf("Abbreviation: want BPC, got %q", got.Abbreviation)
	}
	if got.FullName != "Customers" {
		t.Errorf("FullName: want Customers, got %q", got.FullName)
	}
	if got.Module != "Common Data" {
		t.Errorf("Module: want Common Data, got %q", got.Module)
	}
	if got.DocURL != docBaseURL {
		t.Errorf("DocURL: want %q, got %q", docBaseURL, got.DocURL)
	}
}

// --- GET /tables/{abbr} ---

func TestHandleGetTable_Found(t *testing.T) {
	srv := newServer(serverTables)
	w := get(t, srv, "/tables/BPC")
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}

	var resp tableResponse
	decodeBody(t, w, &resp)

	if resp.Abbreviation != "BPC" {
		t.Errorf("Abbreviation: want BPC, got %q", resp.Abbreviation)
	}
	if resp.FullName != "Customers" {
		t.Errorf("FullName: want Customers, got %q", resp.FullName)
	}
	if resp.DocURL != docBaseURL {
		t.Errorf("DocURL: want %q, got %q", docBaseURL, resp.DocURL)
	}
}

func TestHandleGetTable_CaseInsensitive(t *testing.T) {
	srv := newServer(serverTables)
	for _, abbr := range []string{"bpc", "Bpc", "BPC"} {
		w := get(t, srv, "/tables/"+abbr)
		if w.Code != http.StatusOK {
			t.Errorf("[%s] want 200, got %d", abbr, w.Code)
		}
	}
}

func TestHandleGetTable_NotFound(t *testing.T) {
	srv := newServer(serverTables)
	w := get(t, srv, "/tables/ZZUNKNOWN")
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}

	var resp map[string]string
	decodeBody(t, w, &resp)
	if resp["error"] == "" {
		t.Error("expected error field in 404 body")
	}
}

// --- POST /decode ---

func TestHandleDecode_SimpleQuery(t *testing.T) {
	srv := newServer(serverTables)
	w := post(t, srv, "/decode", decodeRequest{SQL: "SELECT * FROM BPC"})
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}

	var resp decodeResponse
	decodeBody(t, w, &resp)

	if resp.Original != "SELECT * FROM BPC" {
		t.Errorf("Original: want %q, got %q", "SELECT * FROM BPC", resp.Original)
	}
	want := "SELECT * FROM Customers [BPC]"
	if resp.Decoded != want {
		t.Errorf("Decoded: want %q, got %q", want, resp.Decoded)
	}
	if len(resp.Tables) != 1 || resp.Tables[0].Abbreviation != "BPC" {
		t.Errorf("Tables: expected [BPC], got %v", resp.Tables)
	}
}

func TestHandleDecode_MultipleJoins(t *testing.T) {
	srv := newServer(serverTables)
	w := post(t, srv, "/decode", decodeRequest{
		SQL: "SELECT * FROM BPC JOIN BPS ON 1=1 JOIN BAL ON 1=1",
	})

	var resp decodeResponse
	decodeBody(t, w, &resp)

	if len(resp.Tables) != 3 {
		t.Errorf("want 3 tables, got %d", len(resp.Tables))
	}
}

func TestHandleDecode_TablesIncludeDocURL(t *testing.T) {
	srv := newServer(serverTables)
	w := post(t, srv, "/decode", decodeRequest{SQL: "SELECT * FROM BPC"})

	var resp decodeResponse
	decodeBody(t, w, &resp)

	if len(resp.Tables) == 0 {
		t.Fatal("expected at least one table in response")
	}
	if resp.Tables[0].DocURL != docBaseURL {
		t.Errorf("DocURL: want %q, got %q", docBaseURL, resp.Tables[0].DocURL)
	}
}

func TestHandleDecode_UnknownTableNoMatch(t *testing.T) {
	srv := newServer(serverTables)
	w := post(t, srv, "/decode", decodeRequest{SQL: "SELECT * FROM ZZUNKNOWN"})
	if w.Code != http.StatusOK {
		t.Errorf("want 200 even for unknown tables, got %d", w.Code)
	}

	var resp decodeResponse
	decodeBody(t, w, &resp)

	if len(resp.Tables) != 0 {
		t.Errorf("want 0 matched tables, got %d", len(resp.Tables))
	}
	if resp.Decoded != resp.Original {
		t.Error("decoded should equal original when no tables match")
	}
}

func TestHandleDecode_EmptySQL(t *testing.T) {
	srv := newServer(serverTables)
	w := post(t, srv, "/decode", decodeRequest{SQL: ""})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for empty sql, got %d", w.Code)
	}
}

func TestHandleDecode_InvalidJSON(t *testing.T) {
	srv := newServer(serverTables)
	r := httptest.NewRequest(http.MethodPost, "/decode", bytes.NewBufferString("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid JSON, got %d", w.Code)
	}
}
