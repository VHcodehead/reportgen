// Package dataset loads tabular data from CSV or JSON files and validates it
// before anything downstream tries to do arithmetic on it.
package dataset

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Dataset is a rectangular table: every row has exactly len(Columns) fields.
// That invariant is established by Load and relied on everywhere else.
type Dataset struct {
	Columns []string
	Rows    [][]string
}

// ErrNoRows is returned when a file parses correctly but contains no data rows.
// A header-only file is almost always a truncated export rather than an
// intentionally empty report, so it is treated as an error and not a zero result.
var ErrNoRows = errors.New("file contains a header but no data rows")

// Load reads a .csv or .json file and returns a validated Dataset.
func Load(path string) (*Dataset, error) {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".csv":
		return loadCSV(path)
	case ".json":
		return loadJSON(path)
	default:
		return nil, fmt.Errorf("unsupported input format %q (want .csv or .json)", ext)
	}
}

func loadCSV(path string) (*Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Leave FieldsPerRecord at its default so encoding/csv reports ragged rows
	// itself, with the line number attached.
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("parsing %s: file is empty", filepath.Base(path))
	}
	return newDataset(records[0], records[1:])
}

// loadJSON accepts an array of objects — the shape almost every "export to JSON"
// button produces. Column order is taken from the first object, and any key
// missing from a later object becomes an empty cell rather than an error,
// because sparse exports are common and recoverable.
func loadJSON(path string) (*Dataset, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var objects []map[string]any
	if err := json.Unmarshal(b, &objects); err != nil {
		return nil, fmt.Errorf("parsing %s: expected an array of objects: %w", filepath.Base(path), err)
	}
	if len(objects) == 0 {
		return nil, ErrNoRows
	}

	columns := make([]string, 0, len(objects[0]))
	for k := range objects[0] {
		columns = append(columns, k)
	}
	sortStrings(columns)

	rows := make([][]string, 0, len(objects))
	for _, obj := range objects {
		row := make([]string, len(columns))
		for i, col := range columns {
			row[i] = scalarToString(obj[col])
		}
		rows = append(rows, row)
	}
	return newDataset(columns, rows)
}

func newDataset(header []string, rows [][]string) (*Dataset, error) {
	if len(header) == 0 {
		return nil, errors.New("no columns found")
	}
	seen := make(map[string]bool, len(header))
	columns := make([]string, len(header))
	for i, h := range header {
		h = strings.TrimSpace(h)
		if h == "" {
			return nil, fmt.Errorf("column %d has an empty name", i+1)
		}
		if seen[h] {
			return nil, fmt.Errorf("duplicate column name %q", h)
		}
		seen[h] = true
		columns[i] = h
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	for i, row := range rows {
		if len(row) != len(columns) {
			return nil, fmt.Errorf("row %d has %d fields, want %d", i+2, len(row), len(columns))
		}
	}
	return &Dataset{Columns: columns, Rows: rows}, nil
}

// ColumnIndex returns the position of a named column, or -1.
func (d *Dataset) ColumnIndex(name string) int {
	for i, c := range d.Columns {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

// IsNumeric reports whether every non-empty cell in a column parses as a number.
// Currency symbols, thousands separators and surrounding spaces are tolerated,
// since real exports are rarely clean.
func (d *Dataset) IsNumeric(col int) bool {
	saw := false
	for _, row := range d.Rows {
		cell := strings.TrimSpace(row[col])
		if cell == "" {
			continue
		}
		if _, err := ParseNumber(cell); err != nil {
			return false
		}
		saw = true
	}
	return saw
}

// HasNumericValues reports whether at least one non-empty cell in the column
// parses as a number.
//
// This is deliberately weaker than IsNumeric. Detection needs to be strict: a
// column is only auto-selected, or written into the sheet as a number, when
// every value is clean. Aggregation needs to be lenient: a premium column with
// one "n/a" in it is still a premium column, and the right response is to
// summarize the other rows and report how many were skipped, not to refuse the
// whole file.
func (d *Dataset) HasNumericValues(col int) bool {
	for _, row := range d.Rows {
		if _, err := ParseNumber(row[col]); err == nil {
			return true
		}
	}
	return false
}

// NumericColumns returns the indices of every column that holds numbers.
func (d *Dataset) NumericColumns() []int {
	var out []int
	for i := range d.Columns {
		if d.IsNumeric(i) {
			out = append(out, i)
		}
	}
	return out
}

// TextColumns returns columns suitable for grouping: non-numeric, and with
// fewer distinct values than rows, so grouping actually aggregates something.
func (d *Dataset) TextColumns() []int {
	var out []int
	for i := range d.Columns {
		if d.IsNumeric(i) {
			continue
		}
		distinct := make(map[string]bool)
		for _, row := range d.Rows {
			distinct[row[i]] = true
		}
		if len(distinct) < len(d.Rows) {
			out = append(out, i)
		}
	}
	return out
}

// ParseNumber converts a spreadsheet cell to a float, tolerating "$1,234.50".
func ParseNumber(s string) (float64, error) {
	cleaned := strings.NewReplacer("$", "", ",", "", " ", "", "%", "").Replace(strings.TrimSpace(s))
	if cleaned == "" {
		return 0, errors.New("empty value")
	}
	if strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")") {
		// Accounting notation for negatives: (1,200.00)
		cleaned = "-" + strings.TrimSuffix(strings.TrimPrefix(cleaned, "("), ")")
	}
	return strconv.ParseFloat(cleaned, 64)
}

func scalarToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
