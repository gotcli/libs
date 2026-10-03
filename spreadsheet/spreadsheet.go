package spreadsheet

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

const DefaultSheetName = "Sheet1"

// ReadExcel reads all rows from one worksheet. An empty sheet name selects
// the first worksheet in the workbook.
func ReadExcel(reader io.Reader, sheet string) ([][]string, error) {
	result := make([][]string, 0)
	err := WalkExcel(reader, sheet, func(_ int, row []string) error {
		result = append(result, append([]string(nil), row...))
		return nil
	})
	return result, err
}

// WalkExcel streams worksheet rows to fn. Row numbers are one-based.
func WalkExcel(reader io.Reader, sheet string, fn func(rowNumber int, row []string) error) error {
	if reader == nil {
		return errors.New("Excel reader is required")
	}
	if fn == nil {
		return errors.New("Excel row handler is required")
	}
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return fmt.Errorf("open Excel workbook: %w", err)
	}
	defer file.Close()

	sheet, err = resolveSheet(file, sheet)
	if err != nil {
		return err
	}
	rows, err := file.Rows(sheet)
	if err != nil {
		return fmt.Errorf("open worksheet %q: %w", sheet, err)
	}
	defer rows.Close()

	rowNumber := 0
	for rows.Next() {
		rowNumber++
		columns, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("read worksheet %q row %d: %w", sheet, rowNumber, err)
		}
		if err := fn(rowNumber, columns); err != nil {
			return fmt.Errorf("handle worksheet %q row %d: %w", sheet, rowNumber, err)
		}
	}
	if err := rows.Error(); err != nil {
		return fmt.Errorf("read worksheet %q: %w", sheet, err)
	}
	return nil
}

// ExportExcel writes rows to an XLSX workbook using Excelize's stream writer.
// Values may be strings, numbers, booleans, time.Time values, or excelize.Cell.
func ExportExcel(writer io.Writer, sheet string, rows [][]any) error {
	return ExportExcelRows(writer, sheet, func(yield func([]any) error) error {
		for _, row := range rows {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	})
}

// ExportExcelRows supports large exports without keeping every row in memory.
func ExportExcelRows(writer io.Writer, sheet string, produce func(yield func([]any) error) error) error {
	if writer == nil {
		return errors.New("Excel writer is required")
	}
	if produce == nil {
		return errors.New("Excel row producer is required")
	}
	if strings.TrimSpace(sheet) == "" {
		sheet = DefaultSheetName
	}
	file := excelize.NewFile()
	defer file.Close()
	if sheet != DefaultSheetName {
		if err := file.SetSheetName(DefaultSheetName, sheet); err != nil {
			return fmt.Errorf("rename worksheet: %w", err)
		}
	}
	stream, err := file.NewStreamWriter(sheet)
	if err != nil {
		return fmt.Errorf("create Excel stream writer: %w", err)
	}
	rowNumber := 0
	yield := func(row []any) error {
		rowNumber++
		cell, err := excelize.CoordinatesToCellName(1, rowNumber)
		if err != nil {
			return fmt.Errorf("create cell for row %d: %w", rowNumber, err)
		}
		if err := stream.SetRow(cell, row); err != nil {
			return fmt.Errorf("write Excel row %d: %w", rowNumber, err)
		}
		return nil
	}
	if err := produce(yield); err != nil {
		return fmt.Errorf("produce Excel rows: %w", err)
	}
	if err := stream.Flush(); err != nil {
		return fmt.Errorf("flush Excel rows: %w", err)
	}
	if err := file.Write(writer); err != nil {
		return fmt.Errorf("write Excel workbook: %w", err)
	}
	return nil
}

// ExportCSV writes UTF-8 CSV records using RFC 4180-compatible encoding.
func ExportCSV(writer io.Writer, rows [][]string) error {
	return ExportCSVRows(writer, func(yield func([]string) error) error {
		for _, row := range rows {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	})
}

// ExportCSVRows supports large exports without keeping every row in memory.
func ExportCSVRows(writer io.Writer, produce func(yield func([]string) error) error) error {
	if writer == nil {
		return errors.New("CSV writer is required")
	}
	if produce == nil {
		return errors.New("CSV row producer is required")
	}
	csvWriter := csv.NewWriter(writer)
	yield := func(row []string) error {
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
		return nil
	}
	if err := produce(yield); err != nil {
		return fmt.Errorf("produce CSV rows: %w", err)
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("flush CSV: %w", err)
	}
	return nil
}

func resolveSheet(file *excelize.File, sheet string) (string, error) {
	if strings.TrimSpace(sheet) != "" {
		if index, err := file.GetSheetIndex(sheet); err != nil {
			return "", fmt.Errorf("find worksheet %q: %w", sheet, err)
		} else if index == -1 {
			return "", fmt.Errorf("worksheet %q does not exist", sheet)
		}
		return sheet, nil
	}
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return "", errors.New("Excel workbook contains no worksheets")
	}
	return sheets[0], nil
}
