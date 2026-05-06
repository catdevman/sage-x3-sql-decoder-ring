package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// tableInfo holds lookup data for a Sage X3 table abbreviation.
type tableInfo struct {
	abbreviation string
	fullName     string
	module       string
}

// parseTables reads a Sage X3 table dictionary from r and returns a map keyed
// by upper-case abbreviation.
//
// Two CSV formats are auto-detected from the header row:
//   - Official export: ..., Abbreviation, Description, Module, ...
//   - Simple format:   abbreviation, full_name
func parseTables(r io.Reader) (map[string]tableInfo, error) {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return nil, err
	}

	colAbbr, colName, colModule := 0, 1, -1
	for i, h := range header {
		switch strings.TrimSpace(strings.ToLower(h)) {
		case "abbreviation":
			colAbbr = i
		case "description":
			colName = i
		case "module":
			colModule = i
		}
	}

	tables := make(map[string]tableInfo)
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if colAbbr >= len(rec) || colName >= len(rec) {
			continue
		}
		abbr := strings.ToUpper(strings.TrimSpace(rec[colAbbr]))
		name := strings.TrimSpace(rec[colName])
		if abbr == "" || name == "" {
			continue
		}
		mod := ""
		if colModule >= 0 && colModule < len(rec) {
			mod = strings.TrimSpace(rec[colModule])
		}
		tables[abbr] = tableInfo{abbreviation: abbr, fullName: name, module: mod}
	}
	return tables, nil
}

// loadTables opens path and delegates to parseTables.
func loadTables(path string) (map[string]tableInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return parseTables(f)
}
