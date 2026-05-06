package main

import (
	"fmt"
	"io"
)

const docBaseURL = "https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm"

// writeReport writes the decoded SQL report to w.
func writeReport(w io.Writer, original, decoded string, found []tableInfo) {
	fmt.Fprintln(w, "=== Original SQL ===")
	fmt.Fprintln(w, original)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "=== Decoded SQL ===")
	fmt.Fprintln(w, decoded)
	fmt.Fprintln(w)

	if len(found) == 0 {
		fmt.Fprintln(w, "No known Sage X3 table abbreviations found.")
		return
	}

	fmt.Fprintln(w, "=== Table Reference ===")
	for _, t := range found {
		if t.module != "" {
			fmt.Fprintf(w, "  %-20s  %-40s  [%s]\n", t.abbreviation, t.fullName, t.module)
		} else {
			fmt.Fprintf(w, "  %-20s  %s\n", t.abbreviation, t.fullName)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "=== Documentation ===\n%s\n\n", docBaseURL)
	fmt.Fprintln(w, "Tip: search for each table abbreviation on the documentation page above.")
}
