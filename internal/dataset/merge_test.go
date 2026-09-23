package dataset

import (
	"strings"
	"testing"
)

func ds(cols string, rows ...string) *Dataset {
	d := &Dataset{Columns: strings.Split(cols, ",")}
	for _, r := range rows {
		d.Rows = append(d.Rows, strings.Split(r, ","))
	}
	return d
}

func TestMergeConcatenatesInOrder(t *testing.T) {
	a := ds("dept,amount", "Eng,1", "Eng,2")
	b := ds("dept,amount", "Fin,3")

	m, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if len(m.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(m.Rows))
	}
	if m.Rows[2][0] != "Fin" || m.Rows[2][1] != "3" {
		t.Errorf("last row = %v, want [Fin 3]", m.Rows[2])
	}
}

func TestMergeRealignsReorderedColumns(t *testing.T) {
	a := ds("dept,amount,plan", "Eng,100,PPO")
	// Same columns, different order and capitalisation — the common case for
	// two exports of the same report pulled a month apart.
	b := ds("Plan,DEPT,Amount", "HDHP,Fin,200")

	m, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	got := m.Rows[1]
	want := []string{"Fin", "200", "HDHP"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("aligned row = %v, want %v", got, want)
		}
	}
	// Canonical column names come from the first dataset.
	if m.Columns[0] != "dept" {
		t.Errorf("columns = %v, want first dataset's order", m.Columns)
	}
}

func TestMergeRejectsMissingColumn(t *testing.T) {
	a := ds("dept,amount", "Eng,1")
	b := ds("dept,total", "Fin,2")

	_, err := Merge(a, b)
	if err == nil {
		t.Fatal("expected an error when a column is missing")
	}
	if !strings.Contains(err.Error(), `"amount"`) {
		t.Errorf("error should name the missing column, got: %v", err)
	}
	if !strings.Contains(err.Error(), "input 2") {
		t.Errorf("error should identify which input failed, got: %v", err)
	}
}

func TestMergeRejectsExtraColumn(t *testing.T) {
	a := ds("dept,amount", "Eng,1")
	b := ds("dept,amount,extra", "Fin,2,x")

	if _, err := Merge(a, b); err == nil {
		t.Fatal("expected an error when a column count differs")
	}
}

func TestMergeSingleIsUnchanged(t *testing.T) {
	a := ds("dept,amount", "Eng,1")

	m, err := Merge(a)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if len(m.Rows) != 1 || m.Rows[0][1] != "1" {
		t.Errorf("single-input merge changed the data: %v", m.Rows)
	}
}

func TestMergeNothingIsAnError(t *testing.T) {
	if _, err := Merge(); err == nil {
		t.Fatal("expected an error for zero inputs")
	}
}
