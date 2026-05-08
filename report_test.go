package main

import (
	"strings"
	"testing"
)

func TestWriteReport_ContainsSections(t *testing.T) {
	found := []tableInfo{
		{abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	}
	var buf strings.Builder
	writeReport(&buf, "SELECT * FROM BPC", "SELECT * FROM Customers [BPC]", found)
	out := buf.String()

	for _, want := range []string{
		"=== Original SQL ===",
		"=== Decoded SQL ===",
		"=== Table Reference ===",
		"=== Documentation ===",
		tableDocURL("BPC"),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestWriteReport_ShowsOriginalAndDecoded(t *testing.T) {
	var buf strings.Builder
	writeReport(&buf, "SELECT * FROM BPC", "SELECT * FROM Customers [BPC]", []tableInfo{
		{abbreviation: "BPC", fullName: "Customers"},
	})
	out := buf.String()
	if !strings.Contains(out, "SELECT * FROM BPC") {
		t.Error("output should contain original query")
	}
	if !strings.Contains(out, "SELECT * FROM Customers [BPC]") {
		t.Error("output should contain decoded query")
	}
}

func TestWriteReport_WithModule(t *testing.T) {
	var buf strings.Builder
	writeReport(&buf, "SELECT 1", "SELECT 1", []tableInfo{
		{abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	})
	out := buf.String()
	if !strings.Contains(out, "[Common Data]") {
		t.Errorf("output should show module, got:\n%s", out)
	}
}

func TestWriteReport_WithoutModule(t *testing.T) {
	var buf strings.Builder
	writeReport(&buf, "SELECT 1", "SELECT 1", []tableInfo{
		{abbreviation: "BPC", fullName: "Customers", module: ""},
	})
	out := buf.String()
	if strings.Contains(out, "[]") {
		t.Error("output should not show empty brackets for missing module")
	}
}

func TestWriteReport_NoTablesFound(t *testing.T) {
	var buf strings.Builder
	writeReport(&buf, "SELECT 1", "SELECT 1", nil)
	out := buf.String()
	if !strings.Contains(out, "No known Sage X3 table abbreviations found.") {
		t.Error("output should include no-tables message")
	}
	if strings.Contains(out, "=== Table Reference ===") {
		t.Error("output should not include Table Reference section when nothing found")
	}
	if strings.Contains(out, "=== Documentation ===") {
		t.Error("output should not include Documentation section when nothing found")
	}
}

func TestWriteReport_MultipleTablesListed(t *testing.T) {
	found := []tableInfo{
		{abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
		{abbreviation: "BPS", fullName: "Suppliers", module: "Common Data"},
		{abbreviation: "BAL", fullName: "General balance", module: "Financials"},
	}
	var buf strings.Builder
	writeReport(&buf, "q", "q", found)
	out := buf.String()
	for _, abbr := range []string{"BPC", "BPS", "BAL"} {
		if !strings.Contains(out, abbr) {
			t.Errorf("output missing abbreviation %q", abbr)
		}
	}
}
