# sage-x3-sql-decoder-ring

Translates Sage X3 SQL queries from cryptic table abbreviations into human-readable names, with links back to the official Sage X3 table dictionary documentation.

Sage X3 stores data in tables named with short abbreviations (e.g. `BPC`, `BPS`, `BOH`). When reading raw SQL — from query tools, logs, or database exports — these names are opaque. This tool looks up each abbreviation and rewrites the query so it's immediately understandable.

## Example

```
$ echo "SELECT * FROM BPC INNER JOIN BPS ON BPC.BPCNUM_0 = BPS.BPSNUM_0 LEFT JOIN BAL ON BAL.BALNUM_0 = BPC.BPCNUM_0" | ./sage-x3-sql-decoder-ring
```

```
=== Original SQL ===
SELECT * FROM BPC INNER JOIN BPS ON BPC.BPCNUM_0 = BPS.BPSNUM_0 LEFT JOIN BAL ON BAL.BALNUM_0 = BPC.BPCNUM_0

=== Decoded SQL ===
SELECT * FROM Customers [BPC] INNER JOIN Suppliers [BPS] ON BPC.BPCNUM_0 = BPS.BPSNUM_0 LEFT JOIN General balance [BAL] ON BAL.BALNUM_0 = BPC.BPCNUM_0

=== Table Reference ===
  BPC                   Customers                                 [Common Data]
  BPS                   Suppliers                                 [Common Data]
  BAL                   General balance                           [Financials]

=== Documentation ===
  BPC                   https://online-help.sagex3.com/erp/12/en-us/Content/MCD/BPC.htm
  BPS                   https://online-help.sagex3.com/erp/12/en-us/Content/MCD/BPS.htm
  BAL                   https://online-help.sagex3.com/erp/12/en-us/Content/MCD/BAL.htm
```

## Installation

Download a pre-built binary from the [releases page](https://github.com/catdevman/sage-x3-sql-decoder-ring/releases), or build from source (requires Go 1.21+):

```bash
git clone https://github.com/catdevman/sage-x3-sql-decoder-ring.git
cd sage-x3-sql-decoder-ring
go build -o sage-x3-sql-decoder-ring .
```

## Usage

**Pipe SQL from stdin:**
```bash
echo "SELECT * FROM BPC JOIN BOH ON BPC.BPCNUM_0 = BOH.BPCORD_0" | ./sage-x3-sql-decoder-ring
```

**Pass SQL as an argument:**
```bash
./sage-x3-sql-decoder-ring "SELECT * FROM BPC JOIN BOH ON BPC.BPCNUM_0 = BOH.BPCORD_0"
```

**Read from a file:**
```bash
./sage-x3-sql-decoder-ring < query.sql
```

**Interactive mode** (Ctrl+D to submit):
```bash
./sage-x3-sql-decoder-ring
```

**Custom dictionary path:**
```bash
./sage-x3-sql-decoder-ring -tables /path/to/your-dictionary.csv "SELECT * FROM BPC"
```

**HTTP server mode:**
```bash
./sage-x3-sql-decoder-ring -serve :8080
```

**LSP server mode** (JSON-RPC over stdio):
```bash
./sage-x3-sql-decoder-ring -lsp
```

The tool recognises table names following `FROM`, `JOIN`, `INNER JOIN`, `LEFT JOIN`, `RIGHT JOIN`, `FULL JOIN`, `CROSS JOIN`, `UPDATE`, and `INTO`.

## Table dictionary

2,481 Sage X3 table mappings sourced from the official documentation are embedded in the binary:

https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm

The `-tables` flag accepts a custom CSV to override the built-in dictionary. Two formats are auto-detected from the header row:

- **Official export** columns: `Abbreviation`, `Description`, `Module`
- **Simple format** columns: `abbreviation`, `full_name`

## How it works

1. Loads the table dictionary (built-in or custom) into a map keyed by abbreviation (case-insensitive).
2. Scans the SQL query for table-introducing keywords using a regular expression.
3. For each matched table name found in the dictionary, rewrites it as `Full Name [ABBR]` — leaving column references like `BPC.BPCNUM_0` untouched so the query remains structurally valid.
4. Prints the original query, the decoded query, a reference table with module info, and a link to the documentation.

---

## HTTP API

Start the server with `-serve :8080`. All lookups are local — no network calls, no external dependencies.

#### `GET /tables`

Returns every known table.

```jsonc
// Response 200
[
  {
    "abbreviation": "BPC",
    "fullName": "Customers",
    "module": "Common Data",
    "docURL": "https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm"
  },
  ...
]
```

#### `GET /tables/{abbr}`

Looks up a single abbreviation.

```jsonc
// Response 200
{
  "abbreviation": "BPC",
  "fullName": "Customers",
  "module": "Common Data",
  "docURL": "https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm"
}

// Response 404
{ "error": "unknown abbreviation: ZZUNKNOWN" }
```

#### `POST /decode`

Decodes an entire SQL string.

```jsonc
// Request
{ "sql": "SELECT * FROM BPC JOIN BPS ON BPC.BPCNUM_0 = BPS.BPSNUM_0" }

// Response 200
{
  "original": "SELECT * FROM BPC JOIN BPS ON BPC.BPCNUM_0 = BPS.BPSNUM_0",
  "decoded":  "SELECT * FROM Customers [BPC] JOIN Suppliers [BPS] ON BPC.BPCNUM_0 = BPS.BPSNUM_0",
  "tables": [
    {
      "abbreviation": "BPC",
      "fullName": "Customers",
      "module": "Common Data",
      "docURL": "https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm"
    },
    {
      "abbreviation": "BPS",
      "fullName": "Suppliers",
      "module": "Common Data",
      "docURL": "https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm"
    }
  ]
}

// Response 400
{ "error": "sql field is required" }
```

### Quick test

```bash
# Start the server
./sage-x3-sql-decoder-ring -serve :8080

# Single lookup
curl http://localhost:8080/tables/BPC

# Full decode
curl -s -X POST http://localhost:8080/decode \
  -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT * FROM BPC JOIN BPS ON 1=1"}' | jq .
```
