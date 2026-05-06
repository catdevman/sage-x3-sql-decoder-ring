package main

import (
	"fmt"
	"os"
	"strings"
)

func Example_decodeSQL() {
	tables := map[string]tableInfo{
		"BPC": {abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
		"BPS": {abbreviation: "BPS", fullName: "Suppliers", module: "Common Data"},
		"BAL": {abbreviation: "BAL", fullName: "General balance", module: "Financials"},
	}

	query := "SELECT * FROM BPC INNER JOIN BPS ON BPC.BPCNUM_0 = BPS.BPSNUM_0 LEFT JOIN BAL ON BAL.BALNUM_0 = BPC.BPCNUM_0"
	decoded, found := decodeSQL(query, tables)

	fmt.Println(decoded)
	fmt.Println()
	for _, t := range found {
		fmt.Printf("%s -> %s (%s)\n", t.abbreviation, t.fullName, t.module)
	}

	// Output:
	// SELECT * FROM Customers [BPC] INNER JOIN Suppliers [BPS] ON BPC.BPCNUM_0 = BPS.BPSNUM_0 LEFT JOIN General balance [BAL] ON BAL.BALNUM_0 = BPC.BPCNUM_0
	//
	// BPC -> Customers (Common Data)
	// BPS -> Suppliers (Common Data)
	// BAL -> General balance (Financials)
}

func Example_decodeSQL_unknownTable() {
	tables := map[string]tableInfo{
		"BPC": {abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	}

	query := "SELECT * FROM BPC JOIN ZZUNKNOWN ON 1=1"
	decoded, found := decodeSQL(query, tables)

	fmt.Println(decoded)
	fmt.Printf("%d table(s) decoded\n", len(found))

	// Output:
	// SELECT * FROM Customers [BPC] JOIN ZZUNKNOWN ON 1=1
	// 1 table(s) decoded
}

func Example_parseTables_officialFormat() {
	csv := `Table V2023R1,V9.0 (P12),V11 (P22),Abbreviation,Description,Module,Activity code
AABREV,,,AAB,Abbreviation,Supervisor,
ABANK,,,ABN,Bank sort codes,Common Data,
`
	tables, _ := parseTables(strings.NewReader(csv))
	fmt.Printf("loaded %d tables\n", len(tables))
	fmt.Printf("ABN: %s [%s]\n", tables["ABN"].fullName, tables["ABN"].module)

	// Output:
	// loaded 2 tables
	// ABN: Bank sort codes [Common Data]
}

func Example_parseTables_simpleFormat() {
	csv := `abbreviation,full_name
BPC,Customers
BPS,Suppliers
`
	tables, _ := parseTables(strings.NewReader(csv))
	fmt.Printf("loaded %d tables\n", len(tables))
	fmt.Printf("BPC: %s\n", tables["BPC"].fullName)

	// Output:
	// loaded 2 tables
	// BPC: Customers
}

func Example_writeReport() {
	found := []tableInfo{
		{abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
		{abbreviation: "BPS", fullName: "Suppliers", module: "Common Data"},
	}

	original := "SELECT * FROM BPC JOIN BPS ON BPC.ID = BPS.ID"
	decoded := "SELECT * FROM Customers [BPC] JOIN Suppliers [BPS] ON BPC.ID = BPS.ID"

	writeReport(os.Stdout, original, decoded, found)

	// Output:
	// === Original SQL ===
	// SELECT * FROM BPC JOIN BPS ON BPC.ID = BPS.ID
	//
	// === Decoded SQL ===
	// SELECT * FROM Customers [BPC] JOIN Suppliers [BPS] ON BPC.ID = BPS.ID
	//
	// === Table Reference ===
	//   BPC                   Customers                                 [Common Data]
	//   BPS                   Suppliers                                 [Common Data]
	//
	// === Documentation ===
	// https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm
	//
	// Tip: search for each table abbreviation on the documentation page above.
}

func Example_writeReport_noTablesFound() {
	writeReport(os.Stdout, "SELECT 1", "SELECT 1", nil)

	// Output:
	// === Original SQL ===
	// SELECT 1
	//
	// === Decoded SQL ===
	// SELECT 1
	//
	// No known Sage X3 table abbreviations found.
}
