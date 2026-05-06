#!/usr/bin/env python3
"""Read LSP-framed messages from stdin and pretty-print each JSON body."""
import sys
import json

buf = sys.stdin.buffer.read()
i = 0
while i < len(buf):
    hdr_end = buf.find(b'\r\n\r\n', i)
    if hdr_end == -1:
        break
    headers = buf[i:hdr_end].decode(errors='replace')
    cl = 0
    for line in headers.split('\r\n'):
        if line.lower().startswith('content-length:'):
            cl = int(line.split(':', 1)[1].strip())
    body_start = hdr_end + 4
    body = buf[body_start:body_start + cl]
    try:
        print(json.dumps(json.loads(body), indent=2))
        print()
    except json.JSONDecodeError:
        print(body.decode(errors='replace'))
        print()
    i = body_start + cl
