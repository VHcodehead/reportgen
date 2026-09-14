// Command reportgen turns a CSV or JSON export into a formatted, multi-sheet
// Excel report with summary statistics, live formulas and a chart.
//
//	reportgen -input testdata/enrollments.csv -group-by department -value monthly_premium
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

func run() error {
	input := flag.String("input", "", "path to the input .csv or .json file (required)")
	output := flag.String("output", "", "path for the generated .xlsx file (default: <input>-report.xlsx)")
	groupBy := flag.String("group-by", "", "column to group the summary by (default: first suitable text column)")
	value := flag.String("value", "", "numeric column to summarize (default: first numeric column)")
	flag.Usage = usage
	flag.Parse()

	// Accept a bare path too, so `reportgen data.csv` works the way a person
	// expects a small CLI to work.
	if *input == "" && flag.NArg() > 0 {
		*input = flag.Arg(0)
	}
	if *input == "" {
		flag.Usage()
		return fmt.Errorf("no input file given")
	}

	d, err := dataset.Load(*input)
	if err != nil {
		return err
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
		outPath = defaultOutputPath(*input)
	}
	if err := report.Write(outPath, d, summary); err != nil {
		return err
	}

	abs, err := filepath.Abs(outPath)
	if err != nil {
		abs = outPath
	}
	fmt.Printf("Read %d rows from %s\n", len(d.Rows), *input)
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

func defaultOutputPath(input string) string {
	ext := filepath.Ext(input)
	return strings.TrimSuffix(input, ext) + "-report.xlsx"
}

func usage() {
	fmt.Fprintf(os.Stderr, `reportgen turns a CSV or JSON export into a formatted Excel report.

Usage:
  reportgen -input FILE [-output FILE] [-group-by COLUMN] [-value COLUMN]
  reportgen FILE

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Example:
  reportgen -input testdata/enrollments.csv -group-by department -value monthly_premium
`)
}
