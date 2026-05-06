#!/usr/bin/env bash
# Manual LSP smoke test. Run from the repo root:
#   ./scripts/lsp-test.sh
#
# Requires: jq (for pretty output)

set -euo pipefail

BIN="${1:-./sage-x3-sql-decoder-ring}"

if [[ ! -x "$BIN" ]]; then
  echo "Binary not found: $BIN — run 'go build' first" >&2
  exit 1
fi

# send <json> — writes a properly framed LSP message to stdout.
# Uses printf '%s' | wc -c for the byte count (safe with multi-byte chars).
send() {
  local body="$1"
  local len
  len=$(printf '%s' "$body" | wc -c)
  printf "Content-Length: %d\r\n\r\n%s" "$len" "$body"
}

# Build the full message stream, then pipe it into the binary in one shot.
# The binary exits when it receives the 'exit' notification.
{
  echo "─── initialize ───────────────────────────────────────" >&2
  send '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'

  echo "─── initialized (notification, no response) ──────────" >&2
  send '{"jsonrpc":"2.0","method":"initialized"}'

  echo "─── textDocument/didOpen ──────────────────────────────" >&2
  send '{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///test.sql","languageId":"sql","version":1,"text":"SELECT * FROM BPC INNER JOIN BPS ON BPC.BPCNUM_0 = BPS.BPSNUM_0"}}}'

  echo "─── textDocument/hover  (cursor on BPC, line 0 char 14) ─" >&2
  send '{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///test.sql"},"position":{"line":0,"character":14}}}'

  echo "─── textDocument/hover  (cursor on whitespace) ────────" >&2
  send '{"jsonrpc":"2.0","id":3,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///test.sql"},"position":{"line":0,"character":8}}}'

  echo "─── textDocument/inlayHint ────────────────────────────" >&2
  send '{"jsonrpc":"2.0","id":4,"method":"textDocument/inlayHint","params":{"textDocument":{"uri":"file:///test.sql"},"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":200}}}}'

  echo "─── shutdown ──────────────────────────────────────────" >&2
  send '{"jsonrpc":"2.0","id":5,"method":"shutdown"}'

  echo "─── exit (notification, server exits) ─────────────────" >&2
  send '{"jsonrpc":"2.0","method":"exit"}'

} | "$BIN" -lsp | python3 "$(dirname "$0")/lsp-parse.py"
