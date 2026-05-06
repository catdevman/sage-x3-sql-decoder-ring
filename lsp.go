package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// JSON-RPC 2.0 error codes.
const (
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
)

// --- JSON-RPC wire types ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // absent on notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- LSP types ---

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type contentChange struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   textDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange        `json:"contentChanges"`
}

type hoverParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     lspPosition            `json:"position"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    *lspRange     `json:"range,omitempty"`
}

type inlayHintParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        lspRange               `json:"range"`
}

type inlayHint struct {
	Position lspPosition `json:"position"`
	Label    string      `json:"label"`
	Kind     int         `json:"kind"`    // 1 = Type
	Tooltip  string      `json:"tooltip,omitempty"`
}

// --- Message framing ---

// readMessage reads one LSP message from r following the
// "Content-Length: N\r\n\r\n<body>" framing defined by the LSP spec.
func readMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // blank line separates headers from body
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			n, err := strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writeMessage frames msg as an LSP message and writes it to w.
func writeMessage(w io.Writer, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// --- LSP server ---

type lspServer struct {
	tables map[string]tableInfo
	mu     sync.RWMutex
	docs   map[string]string // URI -> full text, updated on didOpen/didChange
}

func newLSPServer(tables map[string]tableInfo) *lspServer {
	return &lspServer{
		tables: tables,
		docs:   make(map[string]string),
	}
}

// run is the main stdio loop. It reads JSON-RPC messages from r, dispatches
// them, and writes responses to w. It returns when the client sends exit or r
// reaches EOF.
func (s *lspServer) run(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	for {
		body, err := readMessage(br)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}

		// Notifications have no ID — don't send a response.
		isNotification := len(req.ID) == 0

		if isNotification {
			if req.Method == "exit" {
				return nil
			}
			s.dispatchNotification(req.Method, req.Params)
			continue
		}

		result, rpcErr := s.dispatch(req.Method, req.Params)
		resp := rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
			Error:   rpcErr,
		}
		if err := writeMessage(w, resp); err != nil {
			return err
		}
	}
}

func (s *lspServer) dispatchNotification(method string, params json.RawMessage) {
	switch method {
	case "initialized":
		// no-op
	case "textDocument/didOpen":
		var p didOpenParams
		if json.Unmarshal(params, &p) == nil {
			s.mu.Lock()
			s.docs[p.TextDocument.URI] = p.TextDocument.Text
			s.mu.Unlock()
		}
	case "textDocument/didChange":
		var p didChangeParams
		if json.Unmarshal(params, &p) == nil && len(p.ContentChanges) > 0 {
			s.mu.Lock()
			// Full sync (TextDocumentSyncKind.Full = 1): last change is the whole doc.
			s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			s.mu.Unlock()
		}
	case "textDocument/didClose":
		var p struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
		}
		if json.Unmarshal(params, &p) == nil {
			s.mu.Lock()
			delete(s.docs, p.TextDocument.URI)
			s.mu.Unlock()
		}
	}
}

func (s *lspServer) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return s.initialize()
	case "shutdown":
		return nil, nil
	case "textDocument/hover":
		return s.hover(params)
	case "textDocument/inlayHint":
		return s.inlayHints(params)
	default:
		return nil, &rpcError{Code: rpcMethodNotFound, Message: "method not found: " + method}
	}
}

func (s *lspServer) initialize() (any, *rpcError) {
	return map[string]any{
		"capabilities": map[string]any{
			// Declare utf-8 so character offsets are byte offsets, not UTF-16 code units.
			"positionEncoding": "utf-8",
			"textDocumentSync": map[string]any{
				"openClose": true,
				"change":    1, // TextDocumentSyncKind.Full
			},
			"hoverProvider":     true,
			"inlayHintProvider": true,
		},
		"serverInfo": map[string]string{
			"name":    "sage-x3-sql-decoder-ring",
			"version": "0.1.0",
		},
	}, nil
}

func (s *lspServer) hover(params json.RawMessage) (any, *rpcError) {
	var p hoverParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: rpcInvalidParams, Message: err.Error()}
	}
	s.mu.RLock()
	text, ok := s.docs[p.TextDocument.URI]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	return hoverAt(text, p.Position, s.tables), nil
}

func (s *lspServer) inlayHints(params json.RawMessage) (any, *rpcError) {
	var p inlayHintParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: rpcInvalidParams, Message: err.Error()}
	}
	s.mu.RLock()
	text, ok := s.docs[p.TextDocument.URI]
	s.mu.RUnlock()
	if !ok {
		return []inlayHint{}, nil
	}
	return inlayHintsFor(text, s.tables), nil
}

// --- Pure handler logic (unit-testable without the server) ---

// hoverAt returns a hover result for the identifier at pos within text, or nil
// if the token is not a known Sage X3 table abbreviation.
func hoverAt(text string, pos lspPosition, tables map[string]tableInfo) *hoverResult {
	offset := positionToOffset(text, pos)

	// Expand to SQL identifier word boundaries.
	start := offset
	for start > 0 && isIdentByte(text[start-1]) {
		start--
	}
	end := offset
	for end < len(text) && isIdentByte(text[end]) {
		end++
	}
	if start == end {
		return nil
	}

	abbr := strings.ToUpper(text[start:end])
	t, ok := tables[abbr]
	if !ok {
		return nil
	}

	md := fmt.Sprintf("**%s** (`%s`)", t.fullName, t.abbreviation)
	if t.module != "" {
		md += "\n\nModule: " + t.module
	}
	md += fmt.Sprintf("\n\n[Open documentation](%s)", docBaseURL)

	return &hoverResult{
		Contents: markupContent{Kind: "markdown", Value: md},
		Range: &lspRange{
			Start: offsetToPosition(text, start),
			End:   offsetToPosition(text, end),
		},
	}
}

// inlayHintsFor returns an inlay hint for every table abbreviation that appears
// as a table reference (after FROM/JOIN/etc.) in text.
func inlayHintsFor(text string, tables map[string]tableInfo) []inlayHint {
	matches := tableIntroducers.FindAllStringSubmatchIndex(text, -1)
	var hints []inlayHint
	for _, m := range matches {
		if len(m) < 6 {
			continue
		}
		abbrStart, abbrEnd := m[4], m[5]
		abbr := strings.ToUpper(text[abbrStart:abbrEnd])
		t, ok := tables[abbr]
		if !ok {
			continue
		}
		label := "/* " + t.fullName
		if t.module != "" {
			label += " [" + t.module + "]"
		}
		label += " */"
		hints = append(hints, inlayHint{
			Position: offsetToPosition(text, abbrEnd),
			Label:    label,
			Kind:     1, // InlayHintKind.Type
			Tooltip:  t.fullName + " — " + docBaseURL,
		})
	}
	return hints
}

// --- Position helpers ---

func isIdentByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// offsetToPosition converts a byte offset in text to an LSP Position.
func offsetToPosition(text string, offset int) lspPosition {
	if offset > len(text) {
		offset = len(text)
	}
	line, lineStart := 0, 0
	for i := 0; i < offset; i++ {
		if text[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return lspPosition{Line: line, Character: offset - lineStart}
}

// positionToOffset converts an LSP Position to a byte offset in text.
func positionToOffset(text string, pos lspPosition) int {
	line := 0
	for i := 0; i < len(text); i++ {
		if line == pos.Line {
			offset := i + pos.Character
			if offset > len(text) {
				return len(text)
			}
			return offset
		}
		if text[i] == '\n' {
			line++
		}
	}
	return len(text)
}
