package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

const csvFile = "sage-x3-table-dictionary.csv"

func main() {
	csvPath := flag.String("tables", csvFile, "path to the Sage X3 table dictionary CSV")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-tables path] [sql...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Decodes Sage X3 SQL abbreviations into human-readable table names.\n\n")
		fmt.Fprintf(os.Stderr, "Supply SQL as arguments, pipe it via stdin, or enter it interactively.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	tables, err := loadTables(*csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
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
