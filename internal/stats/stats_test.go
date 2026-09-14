package stats

import (
	"math"
	"testing"

	"github.com/VHcodehead/reportgen/internal/dataset"
)

func fixture() *dataset.Dataset {
	return &dataset.Dataset{
		Columns: []string{"dept", "amount"},
		Rows: [][]string{
			{"Engineering", "100"},
			{"Engineering", "300"},
			{"Finance", "50"},
			{"Finance", "150"},
			{"Finance", "100"},
		},
	}
}

func closeTo(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSummarize(t *testing.T) {
	s, err := Summarize(fixture(), "dept", "amount")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}

	if len(s.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(s.Groups))
	}
	// Engineering totals 400, Finance 300 — largest total sorts first.
	if s.Groups[0].Name != "Engineering" {
		t.Errorf("first group = %q, want Engineering (highest total)", s.Groups[0].Name)
	}

	eng := s.Groups[0]
	if eng.Count != 2 {
		t.Errorf("count = %d, want 2", eng.Count)
	}
	if !closeTo(eng.Sum, 400) {
		t.Errorf("sum = %v, want 400", eng.Sum)
	}
	if !closeTo(eng.Mean, 200) {
		t.Errorf("mean = %v, want 200", eng.Mean)
	}
	if !closeTo(eng.Min, 100) || !closeTo(eng.Max, 300) {
		t.Errorf("min/max = %v/%v, want 100/300", eng.Min, eng.Max)
	}

	if s.Overall.Count != 5 {
		t.Errorf("overall count = %d, want 5", s.Overall.Count)
	}
	if !closeTo(s.Overall.Sum, 700) {
		t.Errorf("overall sum = %v, want 700", s.Overall.Sum)
	}
	if !closeTo(s.Overall.Mean, 140) {
		t.Errorf("overall mean = %v, want 140", s.Overall.Mean)
	}
	if !closeTo(s.Overall.Min, 50) || !closeTo(s.Overall.Max, 300) {
		t.Errorf("overall min/max = %v/%v, want 50/300", s.Overall.Min, s.Overall.Max)
	}
}

func TestSummarizeCountsSkippedRows(t *testing.T) {
	d := &dataset.Dataset{
		Columns: []string{"dept", "amount"},
		Rows: [][]string{
			{"Ops", "100"},
			{"Ops", ""},
			{"Ops", "n/a"},
		},
	}

	s, err := Summarize(d, "dept", "amount")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Skipped != 2 {
		t.Errorf("skipped = %d, want 2", s.Skipped)
	}
	if s.Overall.Count != 1 {
		t.Errorf("counted = %d, want 1", s.Overall.Count)
	}
}

func TestSummarizeLabelsBlankGroups(t *testing.T) {
	d := &dataset.Dataset{
		Columns: []string{"dept", "amount"},
		Rows:    [][]string{{"", "10"}, {"", "20"}},
	}

	s, err := Summarize(d, "dept", "amount")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Groups[0].Name != "(blank)" {
		t.Errorf("group name = %q, want (blank)", s.Groups[0].Name)
	}
}

func TestSummarizeErrors(t *testing.T) {
	d := fixture()

	if _, err := Summarize(d, "nope", "amount"); err == nil {
		t.Error("expected an error for an unknown group column")
	}
	if _, err := Summarize(d, "dept", "nope"); err == nil {
		t.Error("expected an error for an unknown value column")
	}
	if _, err := Summarize(d, "dept", "dept"); err == nil {
		t.Error("expected an error when the value column is not numeric")
	}

	allBlank := &dataset.Dataset{
		Columns: []string{"dept", "amount"},
		Rows:    [][]string{{"Ops", ""}},
	}
	if _, err := Summarize(allBlank, "dept", "amount"); err == nil {
		t.Error("expected an error when no numeric values are present")
	}
}
