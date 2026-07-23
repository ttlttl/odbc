//go:build windows
// +build windows

package api

import "unsafe"

// SQLGetDataBuffer preserves the existing Windows behavior. The native heap
// isolation is required for the legacy Unix InterSystems drivers used by
// GraphNG.
type SQLGetDataBuffer struct{}

func NewSQLGetDataBuffer() *SQLGetDataBuffer {
	return &SQLGetDataBuffer{}
}

func (b *SQLGetDataBuffer) GetData(statementHandle SQLHSTMT, colOrParamNum SQLUSMALLINT, targetType SQLSMALLINT, dst []byte, vallen *SQLLEN) (SQLRETURN, error) {
	var targetValuePtr SQLPOINTER
	if len(dst) > 0 {
		targetValuePtr = SQLPOINTER(unsafe.Pointer(&dst[0]))
	}
	return SQLGetData(statementHandle, colOrParamNum, targetType, targetValuePtr, SQLLEN(len(dst)), vallen), nil
}

func (b *SQLGetDataBuffer) Close() {}
