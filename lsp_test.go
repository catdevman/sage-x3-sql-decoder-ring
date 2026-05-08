package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

// --- helpers ---

var lspTables = map[string]tableInfo{
	"BPC": {abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	"BPS": {abbreviation: "BPS", fullName: "Suppliers", module: "Common Data"},
	"BAL": {abbreviation: "BAL", fullName: "General balance", module: "Financials"},
}

// frame encodes a JSON-RPC message with the LSP Content-Length framing.
func frame(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("frame marshal: %v", err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

// readResponse reads one framed response from r and unmarshals it.
func readResponse(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	body, err := readMessage(r)
	if err != nil {
		t.Fatalf("readResponse: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("readResponse unmarshal: %v", err)
	}
	return m
}

// runSession writes requests to the server and collects responses.
// requests is a slice of request maps (notifications have no "id").
// The function returns responses in the same order as the requests that had IDs.
func runSession(t *testing.T, requests []map[string]any) []map[string]any {
	t.Helper()

	var input bytes.Buffer
	for _, req := range requests {
		input.Write(frame(t, req))
	}

	var output bytes.Buffer
	srv := newLSPServer(lspTables)
	// run reads until EOF on input.
	if err := srv.run(&input, &output); err != nil && err != io.EOF {
		t.Fatalf("run: %v", err)
	}

	br := bufio.NewReader(&output)
	var responses []map[string]any
	for {
		body, err := readMessage(br)
		if err != nil {
			break
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err == nil {
			responses = append(responses, m)
		}
	}
	return responses
}

// --- readMessage / writeMessage ---

func TestReadMessage_ValidFrame(t *testing.T) {
	payload := `{"method":"test"}`
	raw := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
	body, err := readMessage(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != payload {
		t.Errorf("got %q, want %q", body, payload)
	}
}

func TestReadMessage_MissingContentLength(t *testing.T) {
	raw := "X-Custom: foo\r\n\r\n{}"
	_, err := readMessage(bufio.NewReader(strings.NewReader(raw)))
	if err == nil {
		t.Error("expected error for missing Content-Length")
	}
}

func TestWriteReadMessage_RoundTrip(t *testing.T) {
	type msg struct {
		Method string `json:"method"`
		Value  int    `json:"value"`
	}

	var buf bytes.Buffer
	if err := writeMessage(&buf, msg{Method: "test", Value: 42}); err != nil {
		t.Fatalf("write: %v", err)
	}
	body, err := readMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got msg
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Method != "test" || got.Value != 42 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

// --- offsetToPosition / positionToOffset ---

func TestOffsetToPosition_FirstLine(t *testing.T) {
	text := "SELECT * FROM BPC"
	pos := offsetToPosition(text, 14) // 'B' of BPC
	if pos.Line != 0 || pos.Character != 14 {
		t.Errorf("got {%d,%d}, want {0,14}", pos.Line, pos.Character)
	}
}

func TestOffsetToPosition_SecondLine(t *testing.T) {
	text := "SELECT *\nFROM BPC"
	pos := offsetToPosition(text, 14) // 'B' of BPC (offset 14 = 9 + 5)
	if pos.Line != 1 || pos.Character != 5 {
		t.Errorf("got {%d,%d}, want {1,5}", pos.Line, pos.Character)
	}
}

func TestOffsetToPosition_PastEnd(t *testing.T) {
	text := "abc"
	pos := offsetToPosition(text, 999)
	if pos.Line != 0 || pos.Character != 3 {
		t.Errorf("past-end: got {%d,%d}, want {0,3}", pos.Line, pos.Character)
	}
}

func TestPositionToOffset_FirstLine(t *testing.T) {
	text := "SELECT * FROM BPC"
	offset := positionToOffset(text, lspPosition{Line: 0, Character: 14})
	if offset != 14 {
		t.Errorf("got %d, want 14", offset)
	}
}

func TestPositionToOffset_SecondLine(t *testing.T) {
	text := "SELECT *\nFROM BPC"
	offset := positionToOffset(text, lspPosition{Line: 1, Character: 5})
	if offset != 14 {
		t.Errorf("got %d, want 14", offset)
	}
}

func TestOffsetPosition_RoundTrip(t *testing.T) {
	text := "line one\nline two\nline three"
	for offset := 0; offset <= len(text); offset++ {
		pos := offsetToPosition(text, offset)
		back := positionToOffset(text, pos)
		if back != offset {
			t.Errorf("offset %d -> pos %+v -> offset %d", offset, pos, back)
		}
	}
}

// --- hoverAt ---

func TestHoverAt_KnownTable(t *testing.T) {
	text := "SELECT * FROM BPC WHERE 1=1"
	// cursor on 'B' of BPC (offset 14)
	result := hoverAt(text, lspPosition{Line: 0, Character: 14}, lspTables)
	if result == nil {
		t.Fatal("expected hover result, got nil")
	}
	if !strings.Contains(result.Contents.Value, "Customers") {
		t.Errorf("hover should mention Customers, got %q", result.Contents.Value)
	}
	if !strings.Contains(result.Contents.Value, "BPC") {
		t.Errorf("hover should mention BPC, got %q", result.Contents.Value)
	}
	if !strings.Contains(result.Contents.Value, tableDocURL("BPC")) {
		t.Errorf("hover should include doc URL")
	}
	if result.Contents.Kind != "markdown" {
		t.Errorf("contents kind: want markdown, got %q", result.Contents.Kind)
	}
}

func TestHoverAt_RangeCoversToken(t *testing.T) {
	text := "SELECT * FROM BPC"
	result := hoverAt(text, lspPosition{Line: 0, Character: 15}, lspTables) // cursor in middle of BPC
	if result == nil {
		t.Fatal("expected hover result")
	}
	if result.Range == nil {
		t.Fatal("expected range in hover result")
	}
	if result.Range.Start.Character != 14 || result.Range.End.Character != 17 {
		t.Errorf("range: want chars 14-17, got %d-%d",
			result.Range.Start.Character, result.Range.End.Character)
	}
}

func TestHoverAt_UnknownTable(t *testing.T) {
	text := "SELECT * FROM ZZUNKNOWN"
	result := hoverAt(text, lspPosition{Line: 0, Character: 14}, lspTables)
	if result != nil {
		t.Errorf("expected nil for unknown table, got %+v", result)
	}
}

func TestHoverAt_CursorOnWhitespace(t *testing.T) {
	text := "SELECT * FROM BPC"
	// cursor on the space before BPC
	result := hoverAt(text, lspPosition{Line: 0, Character: 13}, lspTables)
	if result != nil {
		t.Errorf("expected nil on whitespace, got %+v", result)
	}
}

func TestHoverAt_WithModule(t *testing.T) {
	text := "SELECT * FROM BAL"
	result := hoverAt(text, lspPosition{Line: 0, Character: 14}, lspTables)
	if result == nil {
		t.Fatal("expected hover result")
	}
	if !strings.Contains(result.Contents.Value, "Financials") {
		t.Errorf("hover should include module, got %q", result.Contents.Value)
	}
}

func TestHoverAt_MultilineDoc(t *testing.T) {
	text := "SELECT *\nFROM BPC\nJOIN BPS ON 1=1"
	// BPC is on line 1, character 5
	result := hoverAt(text, lspPosition{Line: 1, Character: 5}, lspTables)
	if result == nil {
		t.Fatal("expected hover result on line 1")
	}
	if !strings.Contains(result.Contents.Value, "Customers") {
		t.Errorf("expected Customers, got %q", result.Contents.Value)
	}
}

// --- inlayHintsFor ---

func TestInlayHintsFor_SingleTable(t *testing.T) {
	hints := inlayHintsFor("SELECT * FROM BPC", lspTables)
	if len(hints) != 1 {
		t.Fatalf("want 1 hint, got %d", len(hints))
	}
	if !strings.Contains(hints[0].Label, "Customers") {
		t.Errorf("hint label should mention Customers, got %q", hints[0].Label)
	}
	if !strings.Contains(hints[0].Label, "Common Data") {
		t.Errorf("hint label should include module, got %q", hints[0].Label)
	}
	if hints[0].Kind != 1 {
		t.Errorf("hint kind: want 1 (Type), got %d", hints[0].Kind)
	}
}

func TestInlayHintsFor_HintPositionIsAfterToken(t *testing.T) {
	text := "SELECT * FROM BPC"
	hints := inlayHintsFor(text, lspTables)
	if len(hints) != 1 {
		t.Fatalf("want 1 hint, got %d", len(hints))
	}
	// BPC ends at offset 17, which is character 17 on line 0
	if hints[0].Position.Line != 0 || hints[0].Position.Character != 17 {
		t.Errorf("hint position: want {0,17}, got {%d,%d}",
			hints[0].Position.Line, hints[0].Position.Character)
	}
}

func TestInlayHintsFor_MultipleJoins(t *testing.T) {
	text := "SELECT * FROM BPC JOIN BPS ON 1=1 JOIN BAL ON 1=1"
	hints := inlayHintsFor(text, lspTables)
	if len(hints) != 3 {
		t.Fatalf("want 3 hints, got %d", len(hints))
	}
}

func TestInlayHintsFor_UnknownTableNoHint(t *testing.T) {
	hints := inlayHintsFor("SELECT * FROM ZZUNKNOWN", lspTables)
	if len(hints) != 0 {
		t.Errorf("want 0 hints for unknown table, got %d", len(hints))
	}
}

func TestInlayHintsFor_EmptyDoc(t *testing.T) {
	hints := inlayHintsFor("", lspTables)
	if hints != nil {
		t.Errorf("want nil for empty doc, got %v", hints)
	}
}

func TestInlayHintsFor_MultilinePositions(t *testing.T) {
	text := "SELECT *\nFROM BPC\nJOIN BPS ON 1=1"
	hints := inlayHintsFor(text, lspTables)
	if len(hints) != 2 {
		t.Fatalf("want 2 hints, got %d", len(hints))
	}
	// BPC is on line 1, ends at character 8 ("FROM BPC" → BPC ends at index 8 on that line)
	if hints[0].Position.Line != 1 {
		t.Errorf("first hint: want line 1, got line %d", hints[0].Position.Line)
	}
	// BPS is on line 2
	if hints[1].Position.Line != 2 {
		t.Errorf("second hint: want line 2, got line %d", hints[1].Position.Line)
	}
}

func TestInlayHintsFor_TooltipIncludesDocURL(t *testing.T) {
	hints := inlayHintsFor("SELECT * FROM BPC", lspTables)
	if len(hints) == 0 {
		t.Fatal("expected at least one hint")
	}
	if !strings.Contains(hints[0].Tooltip, tableDocURL("BPC")) {
		t.Errorf("tooltip should include doc URL, got %q", hints[0].Tooltip)
	}
}

// --- LSP server integration ---

func TestLSPServer_Initialize(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 1 {
		t.Fatalf("want 1 response, got %d", len(responses))
	}
	caps, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing or wrong type: %v", responses[0]["result"])
	}
	capabilities, ok := caps["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("capabilities missing in initialize result")
	}
	if capabilities["hoverProvider"] != true {
		t.Error("hoverProvider should be true")
	}
	if capabilities["inlayHintProvider"] != true {
		t.Error("inlayHintProvider should be true")
	}
	if capabilities["positionEncoding"] != "utf-8" {
		t.Error("positionEncoding should be utf-8")
	}
}

func TestLSPServer_ShutdownReturnsNull(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "id": 2, "method": "shutdown"},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	shutdown := responses[1]
	if _, hasError := shutdown["error"]; hasError {
		t.Errorf("shutdown should not return an error: %v", shutdown["error"])
	}
}

func TestLSPServer_UnknownMethodReturnsError(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "id": 2, "method": "doesNotExist"},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	errField, ok := responses[1]["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error field, got %v", responses[1])
	}
	if errField["code"] != float64(rpcMethodNotFound) {
		t.Errorf("error code: want %d, got %v", rpcMethodNotFound, errField["code"])
	}
}

func TestLSPServer_HoverFlow(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "method": "initialized"},
		{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///query.sql",
				"languageId": "sql",
				"version":    1,
				"text":       "SELECT * FROM BPC WHERE 1=1",
			},
		}},
		{"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover", "params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///query.sql"},
			"position":     map[string]any{"line": 0, "character": 14},
		}},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 2 {
		t.Fatalf("want 2 responses (initialize + hover), got %d", len(responses))
	}
	result, ok := responses[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("hover result missing or wrong type: %v", responses[1])
	}
	contents, ok := result["contents"].(map[string]any)
	if !ok {
		t.Fatal("hover contents missing")
	}
	if contents["kind"] != "markdown" {
		t.Errorf("kind: want markdown, got %v", contents["kind"])
	}
	value, _ := contents["value"].(string)
	if !strings.Contains(value, "Customers") {
		t.Errorf("hover text should mention Customers, got %q", value)
	}
}

func TestLSPServer_HoverUnknownDocument(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover", "params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///not-opened.sql"},
			"position":     map[string]any{"line": 0, "character": 0},
		}},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	// null result is valid for hover when doc is not open
	if _, hasError := responses[1]["error"]; hasError {
		t.Errorf("hover on unknown doc should not error: %v", responses[1]["error"])
	}
}

func TestLSPServer_InlayHintFlow(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "method": "initialized"},
		{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///query.sql",
				"languageId": "sql",
				"version":    1,
				"text":       "SELECT * FROM BPC JOIN BPS ON 1=1",
			},
		}},
		{"jsonrpc": "2.0", "id": 2, "method": "textDocument/inlayHint", "params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///query.sql"},
			"range": map[string]any{
				"start": map[string]any{"line": 0, "character": 0},
				"end":   map[string]any{"line": 0, "character": 100},
			},
		}},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	hints, ok := responses[1]["result"].([]any)
	if !ok {
		t.Fatalf("inlayHint result should be an array, got %T: %v", responses[1]["result"], responses[1]["result"])
	}
	if len(hints) != 2 {
		t.Errorf("want 2 hints (BPC + BPS), got %d", len(hints))
	}
}

func TestLSPServer_DidChangeSyncsDocument(t *testing.T) {
	responses := runSession(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///q.sql", "languageId": "sql", "version": 1,
				"text": "SELECT 1",
			},
		}},
		// Update doc to one that has a known table.
		{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///q.sql"},
			"contentChanges": []any{
				map[string]any{"text": "SELECT * FROM BPC"},
			},
		}},
		{"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover", "params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///q.sql"},
			"position":     map[string]any{"line": 0, "character": 14},
		}},
		{"jsonrpc": "2.0", "method": "exit"},
	})

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	result, ok := responses[1]["result"].(map[string]any)
	if !ok || result == nil {
		t.Fatal("expected hover result after didChange, got nil")
	}
	contents := result["contents"].(map[string]any)
	if !strings.Contains(contents["value"].(string), "Customers") {
		t.Error("hover should reflect updated document content")
	}
}
