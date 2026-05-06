package main

import (
	"testing"
)

var testTables = map[string]tableInfo{
	"BPC": {abbreviation: "BPC", fullName: "Customers", module: "Common Data"},
	"BPS": {abbreviation: "BPS", fullName: "Suppliers", module: "Common Data"},
	"BAL": {abbreviation: "BAL", fullName: "General balance", module: "Financials"},
	"BOH": {abbreviation: "BOH", fullName: "Header BOMS", module: "Manufacturing"},
	"BAN": {abbreviation: "BAN", fullName: "Bank accounts", module: "Financials"},
}

func TestDecodeSQL_From(t *testing.T) {
	decoded, found := decodeSQL("SELECT * FROM BPC", testTables)
	want := "SELECT * FROM Customers [BPC]"
	if decoded != want {
		t.Errorf("decoded:\n  got  %q\n  want %q", decoded, want)
	}
	if len(found) != 1 || found[0].abbreviation != "BPC" {
		t.Errorf("found: expected [BPC], got %v", found)
	}
}

func TestDecodeSQL_InnerJoin(t *testing.T) {
	q := "SELECT * FROM BPC INNER JOIN BPS ON BPC.ID = BPS.ID"
	decoded, found := decodeSQL(q, testTables)
	wantDecoded := "SELECT * FROM Customers [BPC] INNER JOIN Suppliers [BPS] ON BPC.ID = BPS.ID"
	if decoded != wantDecoded {
		t.Errorf("decoded:\n  got  %q\n  want %q", decoded, wantDecoded)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 found tables, got %d", len(found))
	}
}

func TestDecodeSQL_AllJoinTypes(t *testing.T) {
	cases := []struct {
		keyword string
	}{
		{"JOIN"},
		{"LEFT JOIN"},
		{"RIGHT JOIN"},
		{"FULL JOIN"},
		{"CROSS JOIN"},
	}

	for _, c := range cases {
		q := "SELECT * FROM BPC " + c.keyword + " BPS ON 1=1"
		decoded, found := decodeSQL(q, testTables)

		if len(found) != 2 {
			t.Errorf("[%s] expected 2 found tables, got %d", c.keyword, len(found))
		}
		// Both BPC and BPS must appear decoded.
		if decoded == q {
			t.Errorf("[%s] query was not modified", c.keyword)
		}
	}
}

func TestDecodeSQL_Update(t *testing.T) {
	q := "UPDATE BPC SET BPC.STATUS_0 = 1 WHERE BPC.ID = 42"
	decoded, found := decodeSQL(q, testTables)
	want := "UPDATE Customers [BPC] SET BPC.STATUS_0 = 1 WHERE BPC.ID = 42"
	if decoded != want {
		t.Errorf("decoded:\n  got  %q\n  want %q", decoded, want)
	}
	if len(found) != 1 {
		t.Errorf("expected 1 found table, got %d", len(found))
	}
}

func TestDecodeSQL_Into(t *testing.T) {
	q := "INSERT INTO BPC (ID) VALUES (1)"
	decoded, found := decodeSQL(q, testTables)
	want := "INSERT INTO Customers [BPC] (ID) VALUES (1)"
	if decoded != want {
		t.Errorf("decoded:\n  got  %q\n  want %q", decoded, want)
	}
	if len(found) != 1 {
		t.Errorf("expected 1 found table, got %d", len(found))
	}
}

func TestDecodeSQL_UnknownTableUnchanged(t *testing.T) {
	q := "SELECT * FROM UNKNOWN_TABLE"
	decoded, found := decodeSQL(q, testTables)
	if decoded != q {
		t.Errorf("query with unknown table should be unchanged, got %q", decoded)
	}
	if len(found) != 0 {
		t.Errorf("expected 0 found tables, got %d", len(found))
	}
}

func TestDecodeSQL_MixedKnownAndUnknown(t *testing.T) {
	q := "SELECT * FROM BPC JOIN ZZUNKNOWN ON 1=1"
	decoded, found := decodeSQL(q, testTables)
	if len(found) != 1 || found[0].abbreviation != "BPC" {
		t.Errorf("expected only BPC in found, got %v", found)
	}
	// BPC replaced, ZZUNKNOWN left as-is.
	wantDecoded := "SELECT * FROM Customers [BPC] JOIN ZZUNKNOWN ON 1=1"
	if decoded != wantDecoded {
		t.Errorf("decoded:\n  got  %q\n  want %q", decoded, wantDecoded)
	}
}

func TestDecodeSQL_SameTableMultipleTimesDeduped(t *testing.T) {
	q := "SELECT * FROM BPC JOIN BPC ON 1=1"
	_, found := decodeSQL(q, testTables)
	if len(found) != 1 {
		t.Errorf("duplicate table should appear once in found, got %d", len(found))
	}
}

func TestDecodeSQL_CaseInsensitiveKeyword(t *testing.T) {
	q := "select * from BPC inner join BPS on 1=1"
	_, found := decodeSQL(q, testTables)
	if len(found) != 2 {
		t.Errorf("expected 2 found tables with lower-case SQL keywords, got %d", len(found))
	}
}

func TestDecodeSQL_CaseInsensitiveTableName(t *testing.T) {
	q := "SELECT * FROM bpc"
	decoded, found := decodeSQL(q, testTables)
	want := "SELECT * FROM Customers [BPC]"
	if decoded != want {
		t.Errorf("decoded:\n  got  %q\n  want %q", decoded, want)
	}
	if len(found) != 1 {
		t.Errorf("expected 1 found table, got %d", len(found))
	}
}

func TestDecodeSQL_EmptyQuery(t *testing.T) {
	decoded, found := decodeSQL("", testTables)
	if decoded != "" {
		t.Errorf("expected empty decoded, got %q", decoded)
	}
	if len(found) != 0 {
		t.Errorf("expected 0 found tables, got %d", len(found))
	}
}

func TestDecodeSQL_MultilineQuery(t *testing.T) {
	q := "SELECT *\nFROM BPC\nJOIN BPS ON BPC.ID = BPS.ID"
	_, found := decodeSQL(q, testTables)
	if len(found) != 2 {
		t.Errorf("expected 2 found tables in multi-line query, got %d", len(found))
	}
}

func TestDecodeSQL_FoundOrderIsFirstSeen(t *testing.T) {
	q := "SELECT * FROM BAL JOIN BPS ON 1=1 JOIN BPC ON 1=1"
	_, found := decodeSQL(q, testTables)
	if len(found) != 3 {
		t.Fatalf("expected 3 found tables, got %d", len(found))
	}
	if found[0].abbreviation != "BAL" || found[1].abbreviation != "BPS" || found[2].abbreviation != "BPC" {
		t.Errorf("unexpected order: %v", found)
	}
}
