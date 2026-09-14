// Package stats computes the summary figures that drive the report.
package stats

import (
	"fmt"
	"sort"

	"github.com/VHcodehead/reportgen/internal/dataset"
)

// Group holds the aggregate figures for one category.
type Group struct {
	Name  string
	Count int
	Sum   float64
	Mean  float64
	Min   float64
	Max   float64
}

// Summary is the full result: per-group figures plus the overall totals.
type Summary struct {
	GroupColumn string
	ValueColumn string
	Groups      []Group
	Overall     Group
	// Skipped counts rows whose value cell was blank or unparseable. Reporting
	// this instead of silently dropping the rows is the whole point of an
	// audit-facing tool: the reader needs to know the denominator.
	Skipped int
}

// Summarize groups the dataset by groupCol and aggregates valueCol.
func Summarize(d *dataset.Dataset, groupCol, valueCol string) (*Summary, error) {
	gi := d.ColumnIndex(groupCol)
	if gi < 0 {
		return nil, fmt.Errorf("group column %q not found", groupCol)
	}
	vi := d.ColumnIndex(valueCol)
	if vi < 0 {
		return nil, fmt.Errorf("value column %q not found", valueCol)
	}
	if !d.HasNumericValues(vi) {
		return nil, fmt.Errorf("value column %q contains no numeric values", valueCol)
	}

	byName := make(map[string]*Group)
	var order []string
	s := &Summary{GroupColumn: d.Columns[gi], ValueColumn: d.Columns[vi]}

	for _, row := range d.Rows {
		v, err := dataset.ParseNumber(row[vi])
		if err != nil {
			s.Skipped++
			continue
		}
		name := row[gi]
		if name == "" {
			name = "(blank)"
		}
		g, ok := byName[name]
		if !ok {
			g = &Group{Name: name, Min: v, Max: v}
			byName[name] = g
			order = append(order, name)
		}
		g.Count++
		g.Sum += v
		if v < g.Min {
			g.Min = v
		}
		if v > g.Max {
			g.Max = v
		}

		if s.Overall.Count == 0 {
			s.Overall.Min, s.Overall.Max = v, v
		}
		s.Overall.Count++
		s.Overall.Sum += v
		if v < s.Overall.Min {
			s.Overall.Min = v
		}
		if v > s.Overall.Max {
			s.Overall.Max = v
		}
	}

	if s.Overall.Count == 0 {
		return nil, fmt.Errorf("no numeric values found in column %q", valueCol)
	}

	for _, name := range order {
		g := byName[name]
		g.Mean = g.Sum / float64(g.Count)
		s.Groups = append(s.Groups, *g)
	}
	// Largest total first: the reader's first question is almost always
	// "where is the money", so answer it without making them sort.
	sort.SliceStable(s.Groups, func(i, j int) bool { return s.Groups[i].Sum > s.Groups[j].Sum })

	s.Overall.Name = "All " + s.GroupColumn
	s.Overall.Mean = s.Overall.Sum / float64(s.Overall.Count)
	return s, nil
}
