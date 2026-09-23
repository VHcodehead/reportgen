// Command reportgen turns one or more CSV or JSON exports into a formatted,
// multi-sheet Excel report with summary statistics, live formulas and a chart.
//
//	reportgen -input testdata/enrollments.csv -group-by department -value monthly_premium
//	reportgen testdata/enrollments.csv testdata/enrollments-q2.csv
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VHcodehead/reportgen/internal/dataset"
	"github.com/VHcodehead/reportgen/internal/report"
	"github.com/VHcodehead/reportgen/internal/stats"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "reportgen:", err)
		os.Exit(1)
	}
}

// pathList lets -input be given more than once.
type pathList []string

func (p *pathList) String() string     { return strings.Join(*p, ", ") }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

func run() error {
	var inputs pathList
	flag.Var(&inputs, "input", "path to a .csv or .json file; repeat the flag to merge several files")
	output := flag.String("output", "", "path for the generated .xlsx file (default: <input>-report.xlsx, or merged-report.xlsx for several inputs)")
	groupBy := flag.String("group-by", "", "column to group the summary by (default: first suitable text column)")
	value := flag.String("value", "", "numeric column to summarize (default: first numeric column)")
	flag.Usage = usage

	// Bare paths work too, so `reportgen a.csv b.csv -value amount` behaves the
	// way a person expects a small CLI to behave. The standard flag package
	// stops parsing at the first positional argument, so parse, peel off the
	// leading paths, and parse again until nothing is left.
	args := os.Args[1:]
	for {
		if err := flag.CommandLine.Parse(args); err != nil {
			return err
		}
		rest := flag.Args()
		i := 0
		for i < len(rest) && !strings.HasPrefix(rest[i], "-") {
			inputs = append(inputs, rest[i])
			i++
		}
		if i == len(rest) {
			break
		}
		args = rest[i:]
	}
	if len(inputs) == 0 {
		flag.Usage()
		return fmt.Errorf("no input file given")
	}

	sets := make([]*dataset.Dataset, 0, len(inputs))
	for _, path := range inputs {
		d, err := dataset.Load(path)
		if err != nil {
			return err
		}
		fmt.Printf("Read %d rows from %s\n", len(d.Rows), path)
		sets = append(sets, d)
	}
	d, err := dataset.Merge(sets...)
	if err != nil {
		return err
	}
	if len(sets) > 1 {
		fmt.Printf("Merged %d files into %d rows\n", len(sets), len(d.Rows))
	}

	groupCol, err := resolveGroupColumn(d, *groupBy)
	if err != nil {
		return err
	}
	valueCol, err := resolveValueColumn(d, *value)
	if err != nil {
		return err
	}

	summary, err := stats.Summarize(d, groupCol, valueCol)
	if err != nil {
		return err
	}

	outPath := *output
	if outPath == "" {
		outPath = defaultOutputPath(inputs)
	}
	if err := report.Write(outPath, d, summary); err != nil {
		return err
	}

	abs, err := filepath.Abs(outPath)
	if err != nil {
		abs = outPath
	}
	fmt.Printf("Grouped %q by %q across %d categories\n", valueCol, groupCol, len(summary.Groups))
	if summary.Skipped > 0 {
		fmt.Printf("Skipped %d row(s) with missing or non-numeric %q\n", summary.Skipped, valueCol)
	}
	fmt.Printf("Report written to %s\n", abs)
	return nil
}

// resolveGroupColumn honours an explicit flag, otherwise picks the first text
// column with repeated values — grouping by a column of unique IDs would
// produce one row per record and summarize nothing.
func resolveGroupColumn(d *dataset.Dataset, requested string) (string, error) {
	if requested != "" {
		if d.ColumnIndex(requested) < 0 {
			return "", fmt.Errorf("group column %q not found; available columns: %s", requested, strings.Join(d.Columns, ", "))
		}
		return requested, nil
	}
	candidates := d.TextColumns()
	if len(candidates) == 0 {
		return "", fmt.Errorf("no column is suitable for grouping; pass -group-by explicitly")
	}
	return d.Columns[candidates[0]], nil
}

func resolveValueColumn(d *dataset.Dataset, requested string) (string, error) {
	if requested != "" {
		i := d.ColumnIndex(requested)
		if i < 0 {
			return "", fmt.Errorf("value column %q not found; available columns: %s", requested, strings.Join(d.Columns, ", "))
		}
		if !d.HasNumericValues(i) {
			return "", fmt.Errorf("value column %q contains no numeric values", requested)
		}
		return requested, nil
	}
	numeric := d.NumericColumns()
	if len(numeric) == 0 {
		return "", fmt.Errorf("no numeric column found to summarize; pass -value explicitly")
	}
	return d.Columns[numeric[0]], nil
}

func defaultOutputPath(inputs []string) string {
	if len(inputs) == 1 {
		ext := filepath.Ext(inputs[0])
		return strings.TrimSuffix(inputs[0], ext) + "-report.xlsx"
	}
	return filepath.Join(filepath.Dir(inputs[0]), "merged-report.xlsx")
}

func usage() {
	fmt.Fprintf(os.Stderr, `reportgen turns CSV or JSON exports into a formatted Excel report.

Usage:
  reportgen -input FILE [-input FILE ...] [-output FILE] [-group-by COLUMN] [-value COLUMN]
  reportgen FILE [FILE ...]

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Examples:
  reportgen -input testdata/enrollments.csv -group-by department -value monthly_premium
  reportgen testdata/enrollments.csv testdata/enrollments-q2.csv -value monthly_premium
`)
}
