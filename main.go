package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	csvPath := flag.String("tables", "", "path to a custom Sage X3 table dictionary CSV (overrides the built-in)")
	serveAddr := flag.String("serve", "", "start HTTP server on this address (e.g. :8080)")
	lspMode := flag.Bool("lsp", false, "start LSP server communicating over stdin/stdout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-tables path] [-lsp] [-serve addr] [sql...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Decodes Sage X3 SQL abbreviations into human-readable table names.\n\n")
		fmt.Fprintf(os.Stderr, "Supply SQL as arguments, pipe it via stdin, or enter it interactively.\n")
		fmt.Fprintf(os.Stderr, "Use -lsp to start the Language Server (JSON-RPC over stdio).\n")
		fmt.Fprintf(os.Stderr, "Use -serve to start the HTTP JSON API server.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	var tables map[string]tableInfo
	var err error
	if *csvPath != "" {
		tables, err = loadTables(*csvPath)
	} else {
		tables, err = parseTables(bytes.NewReader(embeddedCSV))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *lspMode {
		if err := newLSPServer(tables).run(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "lsp error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *serveAddr != "" {
		fmt.Fprintf(os.Stderr, "listening on %s\n", *serveAddr)
		if err := http.ListenAndServe(*serveAddr, newServer(tables)); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var query string

	switch {
	case flag.NArg() > 0:
		query = strings.Join(flag.Args(), " ")

	default:
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Println("Enter SQL query (Ctrl+D to finish):")
		}
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		query = strings.Join(lines, "\n")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		fmt.Fprintln(os.Stderr, "no query provided")
		os.Exit(1)
	}

	decoded, found := decodeSQL(query, tables)
	writeReport(os.Stdout, query, decoded, found)
}
