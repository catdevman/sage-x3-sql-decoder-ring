package main

import (
	"strings"
	"testing"
)

func TestParseTables_OfficialFormat(t *testing.T) {
	csv := `Table V2023R1,V9.0 (P12),V11 (P22),Abbreviation,Description,Module,Activity code
AABREV,,,AAB,Abbreviation,Supervisor,
ABANK,,,ABN,Bank sort codes,Common Data,
`
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(tables))
	}

	got := tables["AABREV"]
	if got.abbreviation != "AABREV" {
		t.Errorf("abbreviation: want AABREV, got %q", got.abbreviation)
	}
	if got.fullName != "Abbreviation" {
		t.Errorf("fullName: want Abbreviation, got %q", got.fullName)
	}
	if got.module != "Supervisor" {
		t.Errorf("module: want Supervisor, got %q", got.module)
	}

	if tables["ABANK"].module != "Common Data" {
		t.Errorf("module: want Common Data, got %q", tables["ABANK"].module)
	}
}

func TestParseTables_SimpleFormat(t *testing.T) {
	csv := `abbreviation,full_name
BPC,Customers
BPS,Suppliers
`
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(tables))
	}
	if tables["BPC"].fullName != "Customers" {
		t.Errorf("want Customers, got %q", tables["BPC"].fullName)
	}
	if tables["BPC"].module != "" {
		t.Errorf("simple format should have no module, got %q", tables["BPC"].module)
	}
}

func TestParseTables_AbbrNormalisedToUpper(t *testing.T) {
	csv := `abbreviation,full_name
bpc,Customers
`
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tables["BPC"]; !ok {
		t.Error("expected abbreviation to be stored as upper-case BPC")
	}
	if _, ok := tables["bpc"]; ok {
		t.Error("lower-case key should not exist")
	}
}

func TestParseTables_SkipsEmptyRows(t *testing.T) {
	csv := `abbreviation,full_name
BPC,Customers
,
BAL,General balance
`
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 2 {
		t.Errorf("expected 2 non-empty entries, got %d", len(tables))
	}
}

func TestParseTables_SkipsRowsMissingDescription(t *testing.T) {
	csv := `abbreviation,full_name
BPC,
BPS,Suppliers
`
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tables["BPC"]; ok {
		t.Error("row with empty description should be skipped")
	}
	if len(tables) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(tables))
	}
}

func TestParseTables_DuplicateAbbrLastWins(t *testing.T) {
	csv := `abbreviation,full_name
BPC,First
BPC,Second
`
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tables["BPC"].fullName != "Second" {
		t.Errorf("want Second (last write wins), got %q", tables["BPC"].fullName)
	}
}

func TestParseTables_EmptyInput(t *testing.T) {
	_, err := parseTables(strings.NewReader(""))
	if err == nil {
		t.Error("expected error on empty input, got nil")
	}
}

func TestParseTables_HeaderOnly(t *testing.T) {
	csv := "abbreviation,full_name\n"
	tables, err := parseTables(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("expected 0 entries, got %d", len(tables))
	}
}
