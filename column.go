// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

type BufferLen api.SQLLEN

func normalizeBufferLen(l BufferLen) BufferLen {
	if l <= 0 {
		return l
	}
	if l <= BufferLen(1<<32-1) {
		if signed := int32(uint32(l)); signed < 0 {
			return BufferLen(signed)
		}
	}
	return l
}

func (l *BufferLen) IsNull() bool {
	return normalizeBufferLen(*l) == BufferLen(api.SQL_NULL_DATA)
}

func (l *BufferLen) IsNoTotal() bool {
	return normalizeBufferLen(*l) == BufferLen(api.SQL_NO_TOTAL)
}

func (l *BufferLen) Int() (int, bool) {
	n := normalizeBufferLen(*l)
	if n < 0 {
		return 0, false
	}
	return int(n), true
}

func (l *BufferLen) GetData(h api.SQLHSTMT, idx int, ctype api.SQLSMALLINT, buf []byte, nativeBuffer *api.SQLGetDataBuffer) (api.SQLRETURN, error) {
	return nativeBuffer.GetData(h, api.SQLUSMALLINT(idx+1), ctype, buf, (*api.SQLLEN)(l))
}

func (l *BufferLen) Bind(h api.SQLHSTMT, idx int, ctype api.SQLSMALLINT, buf []byte) api.SQLRETURN {
	return api.SQLBindCol(h, api.SQLUSMALLINT(idx+1), ctype,
		api.SQLPOINTER(unsafe.Pointer(&buf[0])), api.SQLLEN(len(buf)),
		(*api.SQLLEN)(l))
}

func nulTerminatedLen(buf []byte, ctype api.SQLSMALLINT) int {
	switch ctype {
	case api.SQL_C_WCHAR:
		for i := 0; i+1 < len(buf); i += 2 {
			if buf[i] == 0 && buf[i+1] == 0 {
				return i
			}
		}
	case api.SQL_C_CHAR:
		for i := range buf {
			if buf[i] == 0 {
				return i
			}
		}
	}
	return 0
}

// Column provides access to row columns.
type Column interface {
	Name() string
	Bind(h api.SQLHSTMT, idx int) (bool, error)
	Value(h api.SQLHSTMT, idx int) (driver.Value, error)
	DatabaseTypeName() string
	Length() (int64, bool)
	Nullable() (bool, bool)
	PrecisionScale() (int64, int64, bool)
	ScanType() reflect.Type
}

type beforeFetchColumn interface {
	BeforeFetch()
}

func describeColumn(h api.SQLHSTMT, idx int, namebuf []uint16) (namelen int, sqltype api.SQLSMALLINT, size api.SQLULEN, decimal api.SQLSMALLINT, nullable api.SQLSMALLINT, ret api.SQLRETURN) {
	var l api.SQLSMALLINT
	ret = api.SQLDescribeCol(h, api.SQLUSMALLINT(idx+1),
		(*api.SQLWCHAR)(unsafe.Pointer(&namebuf[0])),
		api.SQLSMALLINT(len(namebuf)), &l,
		&sqltype, &size, &decimal, &nullable)
	return int(l), sqltype, size, decimal, nullable, ret
}

// TODO(brainman): did not check for MS SQL timestamp

func NewColumn(h api.SQLHSTMT, idx int, unicodeResults bool, unicodeCType api.SQLSMALLINT) (Column, error) {
	namebuf := make([]uint16, 150)
	namelen, sqltype, size, decimal, nullable, ret := describeColumn(h, idx, namebuf)
	if ret == api.SQL_SUCCESS_WITH_INFO && namelen > len(namebuf) {
		// try again with bigger buffer
		namebuf = make([]uint16, namelen)
		namelen, sqltype, size, decimal, nullable, ret = describeColumn(h, idx, namebuf)
	}
	if IsError(ret) {
		return nil, NewError("SQLDescribeCol", h)
	}
	if namelen > len(namebuf) {
		// still complaining about buffer size
		return nil, errors.New("Failed to allocate column name buffer")
	}
	b := &BaseColumn{
		name:         api.UTF16ToString(namebuf[:namelen]),
		SQLType:      sqltype,
		Size:         size,
		Decimal:      decimal,
		NullableCode: nullable,
	}
	switch sqltype {
	case api.SQL_BIT:
		return NewBindableColumn(b, api.SQL_C_BIT, 1), nil
	case api.SQL_TINYINT, api.SQL_SMALLINT, api.SQL_INTEGER:
		return NewBindableColumn(b, api.SQL_C_LONG, 4), nil
	case api.SQL_BIGINT:
		return NewBindableColumn(b, api.SQL_C_SBIGINT, 8), nil
	case api.SQL_NUMERIC, api.SQL_DECIMAL, api.SQL_FLOAT, api.SQL_REAL, api.SQL_DOUBLE:
		return NewBindableColumn(b, api.SQL_C_DOUBLE, 8), nil
	case api.SQL_TYPE_TIMESTAMP:
		var v api.SQL_TIMESTAMP_STRUCT
		return NewBindableColumn(b, api.SQL_C_TYPE_TIMESTAMP, int(unsafe.Sizeof(v))), nil
	case api.SQL_TYPE_DATE:
		var v api.SQL_DATE_STRUCT
		return NewBindableColumn(b, api.SQL_C_DATE, int(unsafe.Sizeof(v))), nil
	case api.SQL_TYPE_TIME:
		var v api.SQL_TIME_STRUCT
		return NewBindableColumn(b, api.SQL_C_TIME, int(unsafe.Sizeof(v))), nil
	case api.SQL_SS_TIME2:
		var v api.SQL_SS_TIME2_STRUCT
		return NewBindableColumn(b, api.SQL_C_BINARY, int(unsafe.Sizeof(v))), nil
	case api.SQL_GUID:
		var v api.SQLGUID
		return NewBindableColumn(b, api.SQL_C_GUID, int(unsafe.Sizeof(v))), nil
	case api.SQL_CHAR, api.SQL_VARCHAR:
		if unicodeResults {
			return NewNonBindableColumn(b, unicodeCType), nil
		}
		return NewVariableWidthColumn(b, api.SQL_C_CHAR, size)
	case api.SQL_WCHAR, api.SQL_WVARCHAR:
		if unicodeResults {
			return NewNonBindableColumn(b, unicodeCType), nil
		}
		return NewVariableWidthColumn(b, api.SQL_C_WCHAR, size)
	case api.SQL_BINARY, api.SQL_VARBINARY:
		return NewVariableWidthColumn(b, api.SQL_C_BINARY, size)
	case api.SQL_LONGVARCHAR:
		if unicodeResults {
			return NewNonBindableColumn(b, unicodeCType), nil
		}
		return NewVariableWidthColumn(b, api.SQL_C_CHAR, 0)
	case api.SQL_WLONGVARCHAR, api.SQL_SS_XML:
		if unicodeResults {
			return NewNonBindableColumn(b, unicodeCType), nil
		}
		return NewVariableWidthColumn(b, api.SQL_C_WCHAR, 0)
	case api.SQL_LONGVARBINARY:
		return NewVariableWidthColumn(b, api.SQL_C_BINARY, 0)
	default:
		return nil, fmt.Errorf("unsupported column type %d", sqltype)
	}
}

// BaseColumn implements common column functionality.
type BaseColumn struct {
	name         string
	SQLType      api.SQLSMALLINT
	CType        api.SQLSMALLINT
	Size         api.SQLULEN
	Decimal      api.SQLSMALLINT
	NullableCode api.SQLSMALLINT
}

func (c *BaseColumn) Name() string {
	return c.name
}

func (c *BaseColumn) DatabaseTypeName() string {
	switch c.SQLType {
	case api.SQL_BIT:
		return "BIT"
	case api.SQL_TINYINT:
		return "TINYINT"
	case api.SQL_SMALLINT:
		return "SMALLINT"
	case api.SQL_INTEGER:
		return "INTEGER"
	case api.SQL_BIGINT:
		return "BIGINT"
	case api.SQL_NUMERIC:
		return "NUMERIC"
	case api.SQL_DECIMAL:
		return "DECIMAL"
	case api.SQL_FLOAT:
		return "FLOAT"
	case api.SQL_REAL:
		return "REAL"
	case api.SQL_DOUBLE:
		return "DOUBLE"
	case api.SQL_TYPE_TIMESTAMP, api.SQL_TIMESTAMP:
		return "TIMESTAMP"
	case api.SQL_TYPE_DATE, api.SQL_DATE:
		return "DATE"
	case api.SQL_TYPE_TIME, api.SQL_TIME, api.SQL_SS_TIME2:
		return "TIME"
	case api.SQL_CHAR:
		return "CHAR"
	case api.SQL_VARCHAR:
		return "VARCHAR"
	case api.SQL_WCHAR:
		return "WCHAR"
	case api.SQL_WVARCHAR:
		return "WVARCHAR"
	case api.SQL_LONGVARCHAR:
		return "LONGVARCHAR"
	case api.SQL_WLONGVARCHAR:
		return "WLONGVARCHAR"
	case api.SQL_BINARY:
		return "BINARY"
	case api.SQL_VARBINARY:
		return "VARBINARY"
	case api.SQL_LONGVARBINARY:
		return "LONGVARBINARY"
	case api.SQL_GUID:
		return "GUID"
	case api.SQL_SS_XML:
		return "XML"
	default:
		return ""
	}
}

func (c *BaseColumn) Length() (int64, bool) {
	switch c.SQLType {
	case api.SQL_CHAR, api.SQL_VARCHAR, api.SQL_WCHAR, api.SQL_WVARCHAR,
		api.SQL_LONGVARCHAR, api.SQL_WLONGVARCHAR,
		api.SQL_BINARY, api.SQL_VARBINARY, api.SQL_LONGVARBINARY:
		return int64(c.Size), true
	default:
		return 0, false
	}
}

func (c *BaseColumn) Nullable() (bool, bool) {
	switch c.NullableCode {
	case 0:
		return false, true
	case 1:
		return true, true
	default:
		return false, false
	}
}

func (c *BaseColumn) PrecisionScale() (int64, int64, bool) {
	switch c.SQLType {
	case api.SQL_NUMERIC, api.SQL_DECIMAL:
		return int64(c.Size), int64(c.Decimal), true
	default:
		return 0, 0, false
	}
}

func (c *BaseColumn) ScanType() reflect.Type {
	switch c.CType {
	case api.SQL_C_BIT:
		return reflect.TypeOf(false)
	case api.SQL_C_LONG:
		return reflect.TypeOf(int32(0))
	case api.SQL_C_SBIGINT:
		return reflect.TypeOf(int64(0))
	case api.SQL_C_DOUBLE:
		return reflect.TypeOf(float64(0))
	case api.SQL_C_CHAR, api.SQL_C_WCHAR:
		return reflect.TypeOf("")
	case api.SQL_C_TYPE_TIMESTAMP, api.SQL_C_DATE, api.SQL_C_TIME:
		return reflect.TypeOf(time.Time{})
	case api.SQL_C_BINARY:
		if c.SQLType == api.SQL_SS_TIME2 {
			return reflect.TypeOf(time.Time{})
		}
		return reflect.TypeOf([]byte{})
	case api.SQL_C_GUID:
		return reflect.TypeOf("")
	default:
		return reflect.TypeOf(new(interface{})).Elem()
	}
}

func (c *BaseColumn) Value(buf []byte) (driver.Value, error) {
	var p unsafe.Pointer
	if len(buf) > 0 {
		p = unsafe.Pointer(&buf[0])
	}
	switch c.CType {
	case api.SQL_C_BIT:
		return buf[0] != 0, nil
	case api.SQL_C_LONG:
		return *((*int32)(p)), nil
	case api.SQL_C_SBIGINT:
		return *((*int64)(p)), nil
	case api.SQL_C_DOUBLE:
		return *((*float64)(p)), nil
	case api.SQL_C_CHAR:
		return string(buf), nil
	case api.SQL_C_WCHAR:
		if p == nil {
			return buf, nil
		}
		s := (*[1 << 28]uint16)(p)[: len(buf)/2 : len(buf)/2]
		return utf16toutf8(s), nil
	case api.SQL_C_TYPE_TIMESTAMP:
		t := (*api.SQL_TIMESTAMP_STRUCT)(p)
		r := time.Date(int(t.Year), time.Month(t.Month), int(t.Day),
			int(t.Hour), int(t.Minute), int(t.Second), int(t.Fraction),
			time.Local)
		return r, nil
	case api.SQL_C_GUID:
		t := (*api.SQLGUID)(p)
		var p1, p2 string
		for _, d := range t.Data4[:2] {
			p1 += fmt.Sprintf("%02x", d)
		}
		for _, d := range t.Data4[2:] {
			p2 += fmt.Sprintf("%02x", d)
		}
		r := fmt.Sprintf("%08x-%04x-%04x-%s-%s",
			t.Data1, t.Data2, t.Data3, p1, p2)
		return r, nil
	case api.SQL_C_DATE:
		t := (*api.SQL_DATE_STRUCT)(p)
		r := time.Date(int(t.Year), time.Month(t.Month), int(t.Day),
			0, 0, 0, 0, time.Local)
		return r, nil
	case api.SQL_C_TIME:
		t := (*api.SQL_TIME_STRUCT)(p)
		r := time.Date(1, time.January, 1,
			int(t.Hour), int(t.Minute), int(t.Second), 0, time.Local)
		return r, nil
	case api.SQL_C_BINARY:
		if c.SQLType == api.SQL_SS_TIME2 {
			t := (*api.SQL_SS_TIME2_STRUCT)(p)
			r := time.Date(1, time.January, 1,
				int(t.Hour), int(t.Minute), int(t.Second), int(t.Fraction),
				time.Local)
			return r, nil
		}
		return buf, nil
	}
	return nil, fmt.Errorf("unsupported column ctype %d", c.CType)
}

// BindableColumn allows access to columns that can have their buffers
// bound. Once bound at start, they are written to by odbc driver every
// time it fetches new row. This saves on syscall and, perhaps, some
// buffer copying. BindableColumn can be left unbound, then it behaves
// like NonBindableColumn when user reads data from it.
type BindableColumn struct {
	*BaseColumn
	IsBound         bool
	IsVariableWidth bool
	Size            int
	Len             BufferLen
	Buffer          []byte
	getDataBuffer   *api.SQLGetDataBuffer
}

// TODO(brainman): BindableColumn.Buffer is used by external code after external code returns - that needs to be avoided in the future

func NewBindableColumn(b *BaseColumn, ctype api.SQLSMALLINT, bufSize int) *BindableColumn {
	b.CType = ctype
	c := &BindableColumn{
		BaseColumn:    b,
		Size:          bufSize,
		getDataBuffer: api.NewSQLGetDataBuffer(),
	}
	l := 8 // always use small starting buffer
	if c.Size > l {
		l = c.Size
	}
	c.Buffer = make([]byte, l)
	return c
}

func NewVariableWidthColumn(b *BaseColumn, ctype api.SQLSMALLINT, colWidth api.SQLULEN) (Column, error) {
	if colWidth == 0 || colWidth > 1024 {
		b.CType = ctype
		return NewNonBindableColumn(b, ctype), nil
	}
	l := int(colWidth)
	switch ctype {
	case api.SQL_C_WCHAR:
		l += 1 // room for null-termination character
		l *= 2 // wchars take 2 bytes each
	case api.SQL_C_CHAR:
		l += 1 // room for null-termination character
	case api.SQL_C_BINARY:
		// nothing to do
	default:
		return nil, fmt.Errorf("do not know how wide column of ctype %d is", ctype)
	}
	if l < 1024 {
		// Some ODBC drivers report a smaller display size for expression columns
		// than the value returned by SQLFetch. Use a larger minimum buffer to
		// keep common short values bound without truncation.
		l = 1024
	}
	c := NewBindableColumn(b, ctype, l)
	c.IsVariableWidth = true
	return c, nil
}

func (c *BindableColumn) Bind(h api.SQLHSTMT, idx int) (bool, error) {
	ret := c.Len.Bind(h, idx, c.CType, c.Buffer)
	if IsError(ret) {
		return false, NewError("SQLBindCol", h)
	}
	c.IsBound = true
	return true, nil
}

func (c *BindableColumn) BeforeFetch() {
	if c.IsVariableWidth {
		for i := range c.Buffer {
			c.Buffer[i] = 0
		}
	}
}

func (c *BindableColumn) Value(h api.SQLHSTMT, idx int) (driver.Value, error) {
	if !c.IsBound {
		ret, err := c.Len.GetData(h, idx, c.CType, c.Buffer, c.getDataBuffer)
		if err != nil {
			return nil, fmt.Errorf("column #%d SQLGetData failed: %w", idx, err)
		}
		if IsError(ret) {
			return nil, NewError("SQLGetData", h)
		}
	}
	if c.Len.IsNull() {
		if n := nulTerminatedLen(c.Buffer, c.CType); n > 0 {
			return c.BaseColumn.Value(c.Buffer[:n])
		}
		// is NULL
		return nil, nil
	}
	n, ok := c.Len.Int()
	if !ok {
		return nil, fmt.Errorf("column #%d returned invalid data length %d", idx, c.Len)
	}
	if !c.IsVariableWidth && n != c.Size {
		return nil, fmt.Errorf("wrong column #%d length %d returned, %d expected", idx, c.Len, c.Size)
	}
	if n > len(c.Buffer) {
		return nil, fmt.Errorf("column #%d data length %d exceeds buffer size %d", idx, c.Len, len(c.Buffer))
	}
	return c.BaseColumn.Value(c.Buffer[:n])
}

func (c *BindableColumn) closeGetDataBuffer() {
	c.getDataBuffer.Close()
}

// NonBindableColumn provide access to columns, that can't be bound.
// These are of character or binary type, and, usually, there is no
// limit for their width.
type NonBindableColumn struct {
	*BaseColumn
	getDataBuffer *api.SQLGetDataBuffer
}

func NewNonBindableColumn(b *BaseColumn, ctype api.SQLSMALLINT) *NonBindableColumn {
	b.CType = ctype
	return &NonBindableColumn{
		BaseColumn:    b,
		getDataBuffer: api.NewSQLGetDataBuffer(),
	}
}

func (c *NonBindableColumn) Bind(h api.SQLHSTMT, idx int) (bool, error) {
	return false, nil
}

func (c *NonBindableColumn) Value(h api.SQLHSTMT, idx int) (driver.Value, error) {
	var l BufferLen
	var total []byte
	var isNull bool
	b := make([]byte, 1024)
loop:
	for {
		ret, err := l.GetData(h, idx, c.CType, b, c.getDataBuffer)
		if err != nil {
			return nil, fmt.Errorf("column #%d SQLGetData failed: %w", idx, err)
		}
		switch ret {
		case api.SQL_SUCCESS:
			done, null, err := c.appendGetDataResult(idx, &total, b, l, false)
			if err != nil {
				return nil, err
			}
			if null {
				isNull = true
			}
			if done {
				break loop
			}
		case api.SQL_SUCCESS_WITH_INFO:
			diagErr := NewError("SQLGetData", h).(*Error)
			truncated, ok := getDataWarningIsNonFatal(diagErr)
			if !ok {
				return nil, diagErr
			}
			if !truncated && getDataLengthExceedsBuffer(l, b) {
				truncated = true
			}
			done, null, err := c.appendGetDataResult(idx, &total, b, l, truncated)
			if err != nil {
				return nil, err
			}
			if null {
				isNull = true
			}
			if done {
				break loop
			}
			if !l.IsNoTotal() {
				b = growGetDataBuffer(b, len(total), l, c.CType)
			}
		default:
			return nil, NewError("SQLGetData", h)
		}
	}
	if isNull {
		return nil, nil
	}
	return c.BaseColumn.Value(total)
}

func (c *NonBindableColumn) closeGetDataBuffer() {
	c.getDataBuffer.Close()
}

func (c *NonBindableColumn) appendGetDataResult(idx int, total *[]byte, b []byte, l BufferLen, truncated bool) (bool, bool, error) {
	if l.IsNull() {
		if n := nulTerminatedLen(b, c.CType); n > 0 {
			*total = append(*total, b[:n]...)
			return true, false, nil
		}
		return true, true, nil
	}
	if truncated {
		n := len(b) - nulTerminatorSize(c.CType)
		if n < 0 {
			n = 0
		}
		*total = append(*total, b[:n]...)
		return false, false, nil
	}
	n, ok := l.Int()
	if !ok {
		if n := nulTerminatedLen(b, c.CType); n > 0 {
			*total = append(*total, b[:n]...)
			return true, false, nil
		}
		return false, false, fmt.Errorf("column #%d returned invalid data length %d", idx, l)
	}
	if n > len(b) {
		return false, false, fmt.Errorf("too much data returned: %d bytes returned, but buffer size is %d", l, cap(b))
	}
	*total = append(*total, b[:n]...)
	return true, false, nil
}

func getDataWarningIsNonFatal(err *Error) (truncated bool, ok bool) {
	if len(err.Diag) == 0 {
		return false, true
	}
	for _, diag := range err.Diag {
		switch diag.State {
		case "01004":
			truncated = true
		case "01S07":
			// Cache Unicode ODBC can report fractional truncation while returning
			// character data through SQLGetData. The returned bytes are still usable.
		default:
			return false, false
		}
	}
	return truncated, true
}

func growGetDataBuffer(b []byte, alreadyRead int, l BufferLen, ctype api.SQLSMALLINT) []byte {
	n, ok := l.Int()
	if !ok {
		return b
	}
	n -= alreadyRead
	n += nulTerminatorSize(ctype)
	if len(b) < n {
		return make([]byte, n)
	}
	return b
}

func getDataLengthExceedsBuffer(l BufferLen, b []byte) bool {
	n, ok := l.Int()
	return ok && n > len(b)
}

func nulTerminatorSize(ctype api.SQLSMALLINT) int {
	switch ctype {
	case api.SQL_C_WCHAR:
		return 2
	case api.SQL_C_CHAR:
		return 1
	default:
		return 0
	}
}
