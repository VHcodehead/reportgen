package dataset

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func TestLoadCSV(t *testing.T) {
	path := writeTemp(t, "in.csv", "dept,amount\nEngineering,100.50\nFinance,200\n")

	d, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(d.Rows), 2; got != want {
		t.Errorf("rows = %d, want %d", got, want)
	}
	if got, want := d.Columns[1], "amount"; got != want {
		t.Errorf("column = %q, want %q", got, want)
	}
	if d.ColumnIndex("DEPT") != 0 {
		t.Error("ColumnIndex should match case-insensitively")
	}
	if d.ColumnIndex("missing") != -1 {
		t.Error("ColumnIndex should return -1 for an unknown column")
	}
}

func TestLoadCSVRejectsRaggedRows(t *testing.T) {
	path := writeTemp(t, "ragged.csv", "a,b\n1,2\n3\n")

	if _, err := Load(path); err == nil {
		t.Fatal("expected an error for a row with the wrong field count")
	}
}

func TestLoadRejectsHeaderOnlyFile(t *testing.T) {
	path := writeTemp(t, "empty.csv", "a,b\n")

	_, err := Load(path)
	if !errors.Is(err, ErrNoRows) {
		t.Fatalf("err = %v, want ErrNoRows", err)
	}
}

func TestLoadRejectsDuplicateColumns(t *testing.T) {
	path := writeTemp(t, "dupes.csv", "a,a\n1,2\n")

	if _, err := Load(path); err == nil {
		t.Fatal("expected an error for duplicate column names")
	}
}

func TestLoadRejectsUnknownExtension(t *testing.T) {
	path := writeTemp(t, "data.txt", "a,b\n1,2\n")

	if _, err := Load(path); err == nil {
		t.Fatal("expected an error for an unsupported extension")
	}
}

func TestLoadJSON(t *testing.T) {
	path := writeTemp(t, "in.json", `[{"dept":"Ops","amount":10.5},{"dept":"Ops","amount":4}]`)

	d, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(d.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(d.Rows))
	}
	if i := d.ColumnIndex("amount"); !d.IsNumeric(i) {
		t.Error("amount should be detected as numeric")
	}
}

func TestIsNumericToleratesFormatting(t *testing.T) {
	path := writeTemp(t, "money.csv", "dept,amount\nA,\"$1,200.00\"\nB,(300.25)\nC,\n")

	d, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !d.IsNumeric(1) {
		t.Error("currency, thousands separators and blanks should still count as numeric")
	}
	if d.IsNumeric(0) {
		t.Error("a text column should not be numeric")
	}
}

func TestTextColumnsExcludesUniqueValues(t *testing.T) {
	// id is unique per row, so grouping by it would summarize nothing.
	path := writeTemp(t, "ids.csv", "id,dept,amount\nx1,Eng,1\nx2,Eng,2\nx3,Fin,3\n")

	d, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, i := range d.TextColumns() {
		if d.Columns[i] == "id" {
			t.Error("a column of unique values should not be offered for grouping")
		}
	}
}

func TestParseNumber(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"1234.5", 1234.5, false},
		{"$1,234.50", 1234.5, false},
		{" 42 ", 42, false},
		{"(300.25)", -300.25, false},
		{"", 0, true},
		{"n/a", 0, true},
	}
	for _, c := range cases {
		got, err := ParseNumber(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseNumber(%q) = %v, want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseNumber(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
