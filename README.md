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
https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm

Tip: search for each table abbreviation on the documentation page above.
```

## Installation

Requires Go 1.21+.

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

**HTTP server mode** (for editor integrations — see [VSCode extension plan](#vscode-extension-plan)):
```bash
./sage-x3-sql-decoder-ring -serve :8080
```

The tool recognises table names following `FROM`, `JOIN`, `INNER JOIN`, `LEFT JOIN`, `RIGHT JOIN`, `FULL JOIN`, `CROSS JOIN`, `UPDATE`, and `INTO`.

## Table dictionary

`sage-x3-table-dictionary.csv` contains 2,481 Sage X3 table mappings sourced from the official documentation at:

https://online-help.sagex3.com/erp/12/en-us/Content/MCD/ATB_0.htm

The CSV uses the format exported from the Sage X3 table dictionary with the following columns:

| Column | Description |
|---|---|
| Table V2023R1 | Physical table name in the database |
| V9.0 (P12) | Table name in Sage X3 v9 |
| V11 (P22) | Table name in Sage X3 v11 |
| Abbreviation | The short abbreviation used in SQL |
| Description | Human-readable table name |
| Module | Sage X3 functional module (e.g. Financials, Common Data) |
| Activity code | Optional activity code controlling feature availability |

The tool also accepts a simpler two-column CSV (`abbreviation,full_name`) and will auto-detect the format from the header row.

## How it works

1. Loads the table dictionary CSV into a map keyed by abbreviation (case-insensitive).
2. Scans the SQL query for table-introducing keywords using a regular expression.
3. For each matched table name found in the dictionary, rewrites it as `Full Name [ABBR]` — leaving column references like `BPC.BPCNUM_0` untouched so the query remains structurally valid.
4. Prints the original query, the decoded query, a reference table with module info, and a link to the documentation.

---

## VSCode extension plan

The server mode (`-serve`) exposes a JSON API designed specifically to support a VSCode extension that overlays human-readable table names as inlay hints and hover tooltips directly in the editor.

### Architecture

```
VSCode Extension
      │
      │  HTTP (localhost)
      ▼
sage-x3-sql-decoder-ring -serve :8080
      │
      │  reads
      ▼
sage-x3-table-dictionary.csv
```

The extension runs the server as a background process on activation and tears it down on deactivation. All lookups are local — no network calls, no external dependencies.

### API reference

#### `GET /tables`

Returns every known table. Useful for seeding an extension-side cache on startup.

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

Looks up a single abbreviation. The extension calls this on demand — e.g. when the cursor moves onto a token — rather than decoding the whole file.

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

Decodes an entire SQL string. The extension sends this when a document is opened or saved.

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

### How the extension uses each endpoint

| VSCode feature | API call | Behaviour |
|---|---|---|
| **Inlay hints** | `POST /decode` on document open/change | Scan result `tables[]` for positions; render `/* Customers */` after each matched token |
| **Hover tooltip** | `GET /tables/{abbr}` on cursor hover | Show full name, module, and a clickable link to `docURL` |
| **Startup cache** | `GET /tables` once on activation | Pre-load all 2,481 entries so hover is instant without a round-trip |

### Extension implementation sketch

```typescript
// On activation
const tables = await fetch('http://localhost:8080/tables').then(r => r.json());
const tableMap = Object.fromEntries(tables.map(t => [t.abbreviation, t]));

// InlayHintsProvider
async provideInlayHints(document, range) {
  const { tables } = await fetch('http://localhost:8080/decode', {
    method: 'POST',
    body: JSON.stringify({ sql: document.getText() }),
  }).then(r => r.json());

  return tables.map(t => {
    const pos = findTokenPosition(document, t.abbreviation);
    return new vscode.InlayHint(pos, `/* ${t.fullName} */`, vscode.InlayHintKind.Type);
  });
}

// HoverProvider
async provideHover(document, position) {
  const word = document.getWordRangeAtPosition(position);
  const abbr = document.getText(word);
  const t = tableMap[abbr.toUpperCase()];
  if (!t) return;

  const md = new vscode.MarkdownString(
    `**${t.fullName}**\n\nModule: ${t.module}\n\n[Open documentation](${t.docURL})`
  );
  return new vscode.Hover(md, word);
}
```

### Running the server for development

```bash
# Start the server
./sage-x3-sql-decoder-ring -serve :8080

# Test single lookup
curl http://localhost:8080/tables/BPC

# Test full decode
curl -s -X POST http://localhost:8080/decode \
  -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT * FROM BPC JOIN BPS ON 1=1"}' | jq .
```
