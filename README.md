# reportgen

A command-line tool that turns a CSV or JSON export into a formatted, multi-sheet Excel report — summary statistics, live formulas, and a chart.

```
reportgen -input testdata/enrollments.csv -group-by department -value monthly_premium
```

```
Read 15 rows from testdata/enrollments.csv
Grouped "monthly_premium" by "department" across 4 categories
Report written to C:\...\testdata\enrollments-report.xlsx
```

The generated workbook has three sheets:

| Sheet | Contents |
|---|---|
| **Summary** | One row per category — count, total, average, min, max — with a TOTAL row built from live Excel formulas |
| **Detailed Data** | Every source row, typed correctly, with a frozen header and autofilter |
| **Charts** | A column chart of totals by category, driven by the Summary sheet |

## Why this sample data

The included fixture is employee benefits enrollment data — departments, plans, premiums, employer contributions, months enrolled. I picked it because it's the shape of data this kind of tool actually meets in the wild: a few hundred rows of people-and-money records where somebody needs a defensible summary by category.

## How to run it

Requires Go 1.24 or newer.

```sh
go test ./...                                    # run the test suite
go build -o reportgen .                          # build the binary
./reportgen -input testdata/enrollments.csv      # generate a report
```

Flags:

| Flag | Meaning |
|---|---|
| `-input` | Path to a `.csv` or `.json` file (also accepted as a bare argument) |
| `-output` | Where to write the `.xlsx` (default: `<input>-report.xlsx`) |
| `-group-by` | Column to group by (default: first text column with repeated values) |
| `-value` | Numeric column to summarize (default: first numeric column) |

## Approach

The program is three small packages behind a thin `main`:

- **`internal/dataset`** — load and validate. Nothing downstream runs until the data is known to be rectangular.
- **`internal/stats`** — pure aggregation. No file or spreadsheet types cross this boundary, so the math is testable on plain structs.
- **`internal/report`** — rendering only. It receives a finished `Summary` and decides nothing about what the numbers mean.

The split is what makes the test suite cheap. `stats` can be exercised against hand-written tables with no fixtures on disk, and `dataset` can be exercised against tiny temp files, so neither test touches Excel at all.

### Decisions worth explaining

**A header-only file is an error, not an empty report.** A CSV with columns and no rows is almost always a truncated export. Returning a valid, empty workbook would hide that; returning `ErrNoRows` makes someone look.

**Skipped rows are counted and printed on the report.** When a value cell is blank or unparseable, the row is excluded from the math — but the count appears both on the console and on the Summary sheet. For anything audit-facing, the reader needs the denominator. Silently dropping rows is how a number becomes indefensible.

**Detection is strict; aggregation is lenient.** These pull in opposite directions and I got it wrong the first time. `IsNumeric` requires *every* value in a column to parse — that's the right rule for auto-selecting a column and for deciding whether to write a cell as a number. But I originally used the same check to guard aggregation, which meant a premium column containing a single `n/a` was rejected outright, and the skipped-row counter could never count anything except blanks. The two features contradicted each other. A unit test caught it. Aggregation now uses `HasNumericValues` — at least one parseable value — so the file is summarized and the bad rows are reported rather than the whole job failing.

**The TOTAL row is formulas, not values.** I could compute the totals in Go and write numbers. Instead the row contains `=SUM(...)`, `=MIN(...)`, `=MAX(...)` and an `=IF(...)`-guarded average. If a reviewer edits a figure, the totals follow — and they can check my arithmetic against Excel's, which is the first thing a careful reviewer does.

**Numeric columns are written as numbers.** A numeric column stored as text is the most common reason a spreadsheet "won't sum". The loader detects numeric columns and the writer preserves the type.

**Number parsing tolerates real-world formatting.** `$1,234.50`, `(300.25)` for accounting-style negatives, and stray whitespace all parse. Exports are rarely clean, and rejecting a file because someone's report had dollar signs in it would make the tool useless on exactly the data it's for.

**Grouping never defaults to a unique-valued column.** Auto-selecting an ID column would produce one row per record and summarize nothing, so `TextColumns` only offers columns whose values repeat.

### Tradeoffs and limits

- **Everything is held in memory.** Fine to the ~100k-row range; beyond that this should stream and aggregate incrementally rather than materializing all rows.
- **One grouping dimension.** Real pivot tables cross two or more. A second dimension would change the Summary sheet from a table into a matrix — a deliberate next step, not an oversight.
- **Column types are inferred, not declared.** A column of ZIP codes or account numbers will read as numeric. A `-text-columns` flag would let a caller override that; I left it out rather than guess at a syntax nobody asked for.
- **JSON input expects an array of objects.** Nested documents would need a flattening rule, and inventing one without a real example seemed worse than failing clearly.

### Testing

`go test ./...` covers the parts where being wrong is expensive: ragged rows, duplicate and empty column names, header-only files, unsupported extensions, currency and accounting number formats, unique-value columns being excluded from grouping, aggregate math, skipped-row counting, blank group labelling, and every error path in `Summarize`.

The Excel writer is deliberately not asserted cell-by-cell. Tests that pin exact spreadsheet coordinates break every time the layout is adjusted and catch almost nothing; the value is in the data and math layers, and that's where the tests are.
