package dataset

import (
	"errors"
	"fmt"
	"strings"
)

// Merge concatenates datasets that describe the same kind of record.
//
// Column order is allowed to differ between inputs — a March export and an
// April export from the same system frequently do — so rows are realigned by
// column name, using the first dataset's order as canonical. A dataset with a
// missing or extra column is an error rather than something to paper over:
// padding the gap with blanks would produce a report whose totals are quietly
// wrong, and nothing downstream could tell.
func Merge(sets ...*Dataset) (*Dataset, error) {
	if len(sets) == 0 {
		return nil, errors.New("nothing to merge")
	}
	first := sets[0]
	out := &Dataset{
		Columns: append([]string(nil), first.Columns...),
		Rows:    make([][]string, 0, totalRows(sets)),
	}
	out.Rows = append(out.Rows, first.Rows...)

	for i, d := range sets[1:] {
		mapping, err := alignColumns(first.Columns, d.Columns)
		if err != nil {
			return nil, fmt.Errorf("input %d: %w", i+2, err)
		}
		for _, row := range d.Rows {
			aligned := make([]string, len(first.Columns))
			for dst, src := range mapping {
				aligned[dst] = row[src]
			}
			out.Rows = append(out.Rows, aligned)
		}
	}
	return out, nil
}

// alignColumns returns, for each canonical column position, the index of the
// matching column in other. Matching is case-insensitive to survive exports
// that disagree about capitalisation.
func alignColumns(canonical, other []string) ([]int, error) {
	if len(other) != len(canonical) {
		return nil, fmt.Errorf("has %d columns, want %d (%s)", len(other), len(canonical), strings.Join(canonical, ", "))
	}
	mapping := make([]int, len(canonical))
	used := make([]bool, len(other))
	for dst, name := range canonical {
		src := -1
		for j, candidate := range other {
			if !used[j] && strings.EqualFold(candidate, name) {
				src = j
				break
			}
		}
		if src < 0 {
			return nil, fmt.Errorf("missing column %q", name)
		}
		used[src] = true
		mapping[dst] = src
	}
	return mapping, nil
}

func totalRows(sets []*Dataset) int {
	n := 0
	for _, d := range sets {
		n += len(d.Rows)
	}
	return n
}
