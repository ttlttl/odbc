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

func assertTypeName(t *testing.T, rows *Rows, index int, want string) {
	t.Helper()
	if got := rows.ColumnTypeDatabaseTypeName(index); got != want {
		t.Fatalf("ColumnTypeDatabaseTypeName(%d) = %q, want %q", index, got, want)
	}
}
