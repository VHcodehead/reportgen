// Package report renders a Summary and its source rows into a formatted
// multi-sheet Excel workbook.
package report

import (
	"fmt"

	"github.com/VHcodehead/reportgen/internal/dataset"
	"github.com/VHcodehead/reportgen/internal/stats"
	"github.com/xuri/excelize/v2"
)

const (
	sheetSummary = "Summary"
	sheetData    = "Detailed Data"
	sheetCharts  = "Charts"
)

// Write builds the workbook and saves it to path.
func Write(path string, d *dataset.Dataset, s *stats.Summary) error {
	f := excelize.NewFile()
	defer f.Close()

	styles, err := newStyles(f)
	if err != nil {
		return err
	}
	if err := writeSummary(f, s, styles); err != nil {
		return err
	}
	if err := writeData(f, d, styles); err != nil {
		return err
	}
	if err := writeChart(f, s); err != nil {
		return err
	}

	// excelize seeds every new file with "Sheet1"; drop it so the workbook
	// opens on Summary with no stray empty tab.
	idx, err := f.GetSheetIndex(sheetSummary)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}
	return f.SaveAs(path)
}

type styleSet struct {
	title, header, group, money, number, totalLabel, totalMoney, totalNumber int
}

func newStyles(f *excelize.File) (*styleSet, error) {
	var s styleSet
	var err error

	if s.title, err = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 15, Color: "1F3864"},
	}); err != nil {
		return nil, err
	}
	if s.header, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F3864"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    box("FFFFFF"),
	}); err != nil {
		return nil, err
	}
	if s.group, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Border: box("D9D9D9"),
	}); err != nil {
		return nil, err
	}
	// 44 is the built-in accounting format; it keeps decimal points aligned
	// down the column, which is what makes a money column readable.
	if s.money, err = f.NewStyle(&excelize.Style{NumFmt: 44, Border: box("D9D9D9")}); err != nil {
		return nil, err
	}
	if s.number, err = f.NewStyle(&excelize.Style{NumFmt: 3, Border: box("D9D9D9")}); err != nil {
		return nil, err
	}
	if s.totalLabel, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Color: "1F3864"},
		Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9E2F3"}},
		Border: box("8EAADB"),
	}); err != nil {
		return nil, err
	}
	if s.totalMoney, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Color: "1F3864"},
		Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9E2F3"}},
		NumFmt: 44, Border: box("8EAADB"),
	}); err != nil {
		return nil, err
	}
	if s.totalNumber, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Color: "1F3864"},
		Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9E2F3"}},
		NumFmt: 3, Border: box("8EAADB"),
	}); err != nil {
		return nil, err
	}
	return &s, nil
}

func box(color string) []excelize.Border {
	sides := []string{"top", "bottom", "left", "right"}
	b := make([]excelize.Border, 0, len(sides))
	for _, side := range sides {
		b = append(b, excelize.Border{Type: side, Color: color, Style: 1})
	}
	return b
}

func writeSummary(f *excelize.File, s *stats.Summary, st *styleSet) error {
	if _, err := f.NewSheet(sheetSummary); err != nil {
		return err
	}
	if err := f.SetCellValue(sheetSummary, "A1", fmt.Sprintf("%s by %s", s.ValueColumn, s.GroupColumn)); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetSummary, "A1", "A1", st.title); err != nil {
		return err
	}
	if err := f.SetCellValue(sheetSummary, "A2", fmt.Sprintf(
		"%d records summarized, %d skipped for missing or non-numeric values",
		s.Overall.Count, s.Skipped)); err != nil {
		return err
	}

	headers := []string{s.GroupColumn, "Count", "Total", "Average", "Minimum", "Maximum"}
	const headerRow = 4
	for i, h := range headers {
		name, err := excelize.CoordinatesToCellName(i+1, headerRow)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheetSummary, name, h); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sheetSummary, "A4", "F4", st.header); err != nil {
		return err
	}
	if err := f.SetRowHeight(sheetSummary, headerRow, 22); err != nil {
		return err
	}

	for i, g := range s.Groups {
		r := headerRow + 1 + i
		f.SetCellValue(sheetSummary, cell("A", r), g.Name)
		f.SetCellValue(sheetSummary, cell("B", r), g.Count)
		f.SetCellValue(sheetSummary, cell("C", r), round2(g.Sum))
		f.SetCellValue(sheetSummary, cell("D", r), round2(g.Mean))
		f.SetCellValue(sheetSummary, cell("E", r), round2(g.Min))
		f.SetCellValue(sheetSummary, cell("F", r), round2(g.Max))

		f.SetCellStyle(sheetSummary, cell("A", r), cell("A", r), st.group)
		f.SetCellStyle(sheetSummary, cell("B", r), cell("B", r), st.number)
		f.SetCellStyle(sheetSummary, cell("C", r), cell("F", r), st.money)
	}

	// The total row uses live formulas rather than values computed in Go.
	// If a reviewer edits a figure above, the totals follow — which is what
	// anyone opening a spreadsheet expects, and it lets them check my math.
	first := headerRow + 1
	last := headerRow + len(s.Groups)
	total := last + 1
	f.SetCellValue(sheetSummary, cell("A", total), "TOTAL")
	f.SetCellFormula(sheetSummary, cell("B", total), fmt.Sprintf("=SUM(B%d:B%d)", first, last))
	f.SetCellFormula(sheetSummary, cell("C", total), fmt.Sprintf("=SUM(C%d:C%d)", first, last))
	f.SetCellFormula(sheetSummary, cell("D", total), fmt.Sprintf("=IF(B%d=0,0,C%d/B%d)", total, total, total))
	f.SetCellFormula(sheetSummary, cell("E", total), fmt.Sprintf("=MIN(E%d:E%d)", first, last))
	f.SetCellFormula(sheetSummary, cell("F", total), fmt.Sprintf("=MAX(F%d:F%d)", first, last))
	f.SetCellStyle(sheetSummary, cell("A", total), cell("A", total), st.totalLabel)
	f.SetCellStyle(sheetSummary, cell("B", total), cell("B", total), st.totalNumber)
	f.SetCellStyle(sheetSummary, cell("C", total), cell("F", total), st.totalMoney)

	f.SetColWidth(sheetSummary, "A", "A", 26)
	f.SetColWidth(sheetSummary, "B", "F", 15)
	return f.SetPanes(sheetSummary, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		YSplit:      headerRow,
		TopLeftCell: "A5",
		ActivePane:  "bottomLeft",
	})
}

func writeData(f *excelize.File, d *dataset.Dataset, st *styleSet) error {
	if _, err := f.NewSheet(sheetData); err != nil {
		return err
	}
	for i, c := range d.Columns {
		name, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheetData, name, c); err != nil {
			return err
		}
	}
	lastCol, err := excelize.ColumnNumberToName(len(d.Columns))
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetData, "A1", lastCol+"1", st.header); err != nil {
		return err
	}
	f.SetRowHeight(sheetData, 1, 22)

	numeric := make(map[int]bool)
	for _, i := range d.NumericColumns() {
		numeric[i] = true
	}
	for r, row := range d.Rows {
		for c, v := range row {
			name, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				return err
			}
			// Write numbers as numbers. A numeric column stored as text is the
			// single most common reason a spreadsheet "won't sum".
			if numeric[c] {
				if n, perr := dataset.ParseNumber(v); perr == nil {
					f.SetCellValue(sheetData, name, n)
					continue
				}
			}
			f.SetCellValue(sheetData, name, v)
		}
	}
	f.SetColWidth(sheetData, "A", lastCol, 18)
	if err := f.AutoFilter(sheetData, fmt.Sprintf("A1:%s%d", lastCol, len(d.Rows)+1), nil); err != nil {
		return err
	}
	return f.SetPanes(sheetData, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
}

func writeChart(f *excelize.File, s *stats.Summary) error {
	if _, err := f.NewSheet(sheetCharts); err != nil {
		return err
	}
	if err := f.SetCellValue(sheetCharts, "A1", fmt.Sprintf("%s by %s", s.ValueColumn, s.GroupColumn)); err != nil {
		return err
	}
	first := 5
	last := 4 + len(s.Groups)
	categories := fmt.Sprintf("%s!$A$%d:$A$%d", quoteSheet(sheetSummary), first, last)
	values := fmt.Sprintf("%s!$C$%d:$C$%d", quoteSheet(sheetSummary), first, last)

	return f.AddChart(sheetCharts, "A3", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("Total %s", s.ValueColumn),
			Categories: categories,
			Values:     values,
		}},
		Title:     []excelize.RichTextRun{{Text: fmt.Sprintf("Total %s by %s", s.ValueColumn, s.GroupColumn)}},
		Legend:    excelize.ChartLegend{Position: "bottom"},
		Dimension: excelize.ChartDimension{Width: 640, Height: 400},
	})
}

// quoteSheet wraps a sheet name in single quotes so chart references survive
// names containing spaces, like "Detailed Data".
func quoteSheet(name string) string {
	return "'" + name + "'"
}

func cell(col string, row int) string { return fmt.Sprintf("%s%d", col, row) }

func round2(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*100+0.5)) / 100
	}
	return float64(int64(v*100+0.5)) / 100
}
