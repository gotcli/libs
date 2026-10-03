package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"reflect"
	"testing"
)

func TestExcelRoundTrip(t *testing.T) {
	rows := [][]any{{"name", "quantity", "active"}, {"LINE OA", 3, true}, {"Booking", 7, false}}
	var workbook bytes.Buffer
	if err := ExportExcel(&workbook, "Products", rows); err != nil {
		t.Fatalf("ExportExcel() error = %v", err)
	}
	got, err := ReadExcel(bytes.NewReader(workbook.Bytes()), "Products")
	if err != nil {
		t.Fatalf("ReadExcel() error = %v", err)
	}
	want := [][]string{{"name", "quantity", "active"}, {"LINE OA", "3", "TRUE"}, {"Booking", "7", "FALSE"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadExcel() = %#v, want %#v", got, want)
	}
}

func TestWalkExcelStopsOnHandlerError(t *testing.T) {
	var workbook bytes.Buffer
	if err := ExportExcel(&workbook, "", [][]any{{"header"}, {"value"}}); err != nil {
		t.Fatal(err)
	}
	want := errors.New("stop")
	err := WalkExcel(bytes.NewReader(workbook.Bytes()), "", func(rowNumber int, _ []string) error {
		if rowNumber == 2 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("WalkExcel() error = %v, want wrapped %v", err, want)
	}
}

func TestExportCSV(t *testing.T) {
	rows := [][]string{{"name", "note"}, {"LINE OA", "hello, world"}, {"ไทย", "บรรทัดใหม่\nได้"}}
	var output bytes.Buffer
	if err := ExportCSV(&output, rows); err != nil {
		t.Fatalf("ExportCSV() error = %v", err)
	}
	got, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("read exported CSV: %v", err)
	}
	if !reflect.DeepEqual(got, rows) {
		t.Fatalf("ExportCSV() = %#v, want %#v", got, rows)
	}
}

func TestStreamingExports(t *testing.T) {
	var excelOutput bytes.Buffer
	err := ExportExcelRows(&excelOutput, "Data", func(yield func([]any) error) error {
		return yield([]any{"streamed", 1})
	})
	if err != nil {
		t.Fatalf("ExportExcelRows() error = %v", err)
	}
	var csvOutput bytes.Buffer
	err = ExportCSVRows(&csvOutput, func(yield func([]string) error) error {
		return yield([]string{"streamed", "1"})
	})
	if err != nil {
		t.Fatalf("ExportCSVRows() error = %v", err)
	}
}
