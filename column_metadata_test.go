package odbc

import (
	"database/sql/driver"
	"reflect"
	"testing"

	"github.com/alexbrainman/odbc/api"
)

var _ driver.RowsColumnTypeScanType = (*Rows)(nil)
var _ driver.RowsColumnTypeDatabaseTypeName = (*Rows)(nil)
var _ driver.RowsColumnTypeLength = (*Rows)(nil)
var _ driver.RowsColumnTypeNullable = (*Rows)(nil)
var _ driver.RowsColumnTypePrecisionScale = (*Rows)(nil)

func TestRowsColumnTypeMetadata(t *testing.T) {
	rows := &Rows{os: &ODBCStmt{Cols: []Column{
		NewBindableColumn(&BaseColumn{name: "id", SQLType: api.SQL_INTEGER, NullableCode: 0}, api.SQL_C_LONG, 4),
		NewBindableColumn(&BaseColumn{name: "amount", SQLType: api.SQL_DECIMAL, Size: 18, Decimal: 2, NullableCode: 1}, api.SQL_C_DOUBLE, 8),
		NewBindableColumn(&BaseColumn{name: "name", SQLType: api.SQL_VARCHAR, Size: 64, NullableCode: 2}, api.SQL_C_CHAR, 65),
	}}}

	if got := rows.Columns(); !reflect.DeepEqual(got, []string{"id", "amount", "name"}) {
		t.Fatalf("Columns() = %#v", got)
	}

	assertTypeName(t, rows, 0, "INTEGER")
	assertTypeName(t, rows, 1, "DECIMAL")
	assertTypeName(t, rows, 2, "VARCHAR")

	if length, ok := rows.ColumnTypeLength(2); !ok || length != 64 {
		t.Fatalf("ColumnTypeLength(2) = (%d, %v), want (64, true)", length, ok)
	}
	if nullable, ok := rows.ColumnTypeNullable(0); !ok || nullable {
		t.Fatalf("ColumnTypeNullable(0) = (%v, %v), want (false, true)", nullable, ok)
	}
	if nullable, ok := rows.ColumnTypeNullable(1); !ok || !nullable {
		t.Fatalf("ColumnTypeNullable(1) = (%v, %v), want (true, true)", nullable, ok)
	}
	if _, ok := rows.ColumnTypeNullable(2); ok {
		t.Fatalf("ColumnTypeNullable(2) ok = true, want false")
	}
	if precision, scale, ok := rows.ColumnTypePrecisionScale(1); !ok || precision != 18 || scale != 2 {
		t.Fatalf("ColumnTypePrecisionScale(1) = (%d, %d, %v), want (18, 2, true)", precision, scale, ok)
	}
}

func TestNewVariableWidthColumnUsesMinimumBuffer(t *testing.T) {
	col, err := NewVariableWidthColumn(&BaseColumn{name: "name", SQLType: api.SQL_VARCHAR, Size: 7}, api.SQL_C_CHAR, 7)
	if err != nil {
		t.Fatal(err)
	}
	bindable, ok := col.(*BindableColumn)
	if !ok {
		t.Fatalf("NewVariableWidthColumn returned %T, want *BindableColumn", col)
	}
	if len(bindable.Buffer) != 1024 {
		t.Fatalf("len(Buffer) = %d, want 1024", len(bindable.Buffer))
	}
}

func TestTextColumnUsesGetDataForUnicodeResults(t *testing.T) {
	col := NewNonBindableColumn(&BaseColumn{name: "name", SQLType: api.SQL_VARCHAR, Size: 7}, api.SQL_C_WCHAR)
	if _, ok := interface{}(col).(*NonBindableColumn); !ok {
		t.Fatalf("NewNonBindableColumn returned %T, want *NonBindableColumn", col)
	}
	if got := col.ScanType(); got != reflect.TypeOf("") {
		t.Fatalf("ScanType() = %v, want string", got)
	}

	col = NewNonBindableColumn(&BaseColumn{name: "wide_name", SQLType: api.SQL_WVARCHAR, Size: 7}, api.SQL_C_WCHAR)
	if _, ok := interface{}(col).(*NonBindableColumn); !ok {
		t.Fatalf("NewNonBindableColumn returned %T, want *NonBindableColumn", col)
	}
}

func TestBufferLenRecognizesUnsigned32BitIndicators(t *testing.T) {
	nullLen := BufferLen(api.SQLLEN(0xffffffff))
	if !nullLen.IsNull() {
		t.Fatalf("IsNull() = false for unsigned 32-bit SQL_NULL_DATA indicator")
	}
	if _, ok := nullLen.Int(); ok {
		t.Fatalf("Int() ok = true for unsigned 32-bit SQL_NULL_DATA indicator")
	}

	noTotalLen := BufferLen(api.SQLLEN(0xfffffffc))
	if !noTotalLen.IsNoTotal() {
		t.Fatalf("IsNoTotal() = false for unsigned 32-bit SQL_NO_TOTAL indicator")
	}
	if _, ok := noTotalLen.Int(); ok {
		t.Fatalf("Int() ok = true for unsigned 32-bit SQL_NO_TOTAL indicator")
	}

	regularLen := BufferLen(api.SQLLEN(1024))
	if regularLen.IsNull() || regularLen.IsNoTotal() {
		t.Fatalf("regular positive length was treated as an indicator")
	}
	if n, ok := regularLen.Int(); !ok || n != 1024 {
		t.Fatalf("Int() = (%d, %v), want (1024, true)", n, ok)
	}
}

func TestNulTerminatedLen(t *testing.T) {
	if got := nulTerminatedLen([]byte{'a', 'b', 'c', 0, 'x'}, api.SQL_C_CHAR); got != 3 {
		t.Fatalf("nulTerminatedLen(char) = %d, want 3", got)
	}
	wide := []byte{'a', 0, 'b', 0, 0, 0, 'x', 0}
	if got := nulTerminatedLen(wide, api.SQL_C_WCHAR); got != 4 {
		t.Fatalf("nulTerminatedLen(wchar) = %d, want 4", got)
	}
	if got := nulTerminatedLen(make([]byte, 8), api.SQL_C_WCHAR); got != 0 {
		t.Fatalf("nulTerminatedLen(empty wchar) = %d, want 0", got)
	}
}

func TestBindableColumnUsesBufferWhenNullIndicatorHasData(t *testing.T) {
	col := NewBindableColumn(&BaseColumn{name: "name", SQLType: api.SQL_VARCHAR, Size: 8}, api.SQL_C_CHAR, 8)
	bindable := col
	bindable.IsBound = true
	bindable.IsVariableWidth = true
	copy(bindable.Buffer, []byte{'a', 'b', 'c', 0})
	bindable.Len = BufferLen(api.SQLLEN(0xffffffff))

	got, err := bindable.Value(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc" {
		t.Fatalf("Value() = %#v, want abc", got)
	}
}

func TestNonBindableColumnUsesBufferWhenIndicatorIsInvalidNegative(t *testing.T) {
	var l BufferLen = -4294967289
	if _, ok := l.Int(); ok {
		t.Fatalf("Int() ok = true for invalid negative indicator")
	}
	if got := nulTerminatedLen([]byte{'S', 'Q', 'L', 'U', 's', 'e', 'r', 0}, api.SQL_C_CHAR); got != 7 {
		t.Fatalf("nulTerminatedLen() = %d, want 7", got)
	}
}

func TestGetDataWarningIsNonFatal(t *testing.T) {
	tests := []struct {
		name          string
		states        []string
		wantTruncated bool
		wantOK        bool
	}{
		{name: "empty", wantOK: true},
		{name: "string truncated", states: []string{"01004"}, wantTruncated: true, wantOK: true},
		{name: "fractional truncation", states: []string{"01S07"}, wantOK: true},
		{name: "mixed nonfatal", states: []string{"01S07", "01004"}, wantTruncated: true, wantOK: true},
		{name: "fatal", states: []string{"22001"}, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{APIName: "SQLGetData"}
			for _, state := range tt.states {
				err.Diag = append(err.Diag, DiagRecord{State: state})
			}
			gotTruncated, gotOK := getDataWarningIsNonFatal(err)
			if gotTruncated != tt.wantTruncated || gotOK != tt.wantOK {
				t.Fatalf("getDataWarningIsNonFatal() = (%v, %v), want (%v, %v)", gotTruncated, gotOK, tt.wantTruncated, tt.wantOK)
			}
		})
	}
}

func TestGetDataLengthExceedsBuffer(t *testing.T) {
	if !getDataLengthExceedsBuffer(BufferLen(2048), make([]byte, 1024)) {
		t.Fatal("getDataLengthExceedsBuffer() = false, want true")
	}
	if getDataLengthExceedsBuffer(BufferLen(1024), make([]byte, 1024)) {
		t.Fatal("getDataLengthExceedsBuffer() = true for exact buffer")
	}
	if getDataLengthExceedsBuffer(BufferLen(api.SQL_NULL_DATA), make([]byte, 1024)) {
		t.Fatal("getDataLengthExceedsBuffer() = true for SQL_NULL_DATA")
	}
}

func TestBindableColumnBeforeFetchClearsVariableWidthBuffer(t *testing.T) {
	col := NewBindableColumn(&BaseColumn{name: "name", SQLType: api.SQL_VARCHAR, Size: 8}, api.SQL_C_CHAR, 8)
	col.IsVariableWidth = true
	copy(col.Buffer, []byte{'a', 'b', 'c', 0})

	col.BeforeFetch()
	if got := nulTerminatedLen(col.Buffer, api.SQL_C_CHAR); got != 0 {
		t.Fatalf("nulTerminatedLen(after BeforeFetch) = %d, want 0", got)
	}
}

func assertTypeName(t *testing.T, rows *Rows, index int, want string) {
	t.Helper()
	if got := rows.ColumnTypeDatabaseTypeName(index); got != want {
		t.Fatalf("ColumnTypeDatabaseTypeName(%d) = %q, want %q", index, got, want)
	}
}
