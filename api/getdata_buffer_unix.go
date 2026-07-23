//go:build (darwin || linux || freebsd) && cgo
// +build darwin linux freebsd
// +build cgo

package api

/*
#include <stdlib.h>
#include <string.h>

static void initializeGuardedBuffer(void *ptr, size_t size, size_t guardSize, unsigned char guardByte) {
	memset(ptr, 0, size);
	memset((unsigned char *)ptr + size, guardByte, guardSize);
}

static int guardedBufferIsValid(void *ptr, size_t size, size_t guardSize, unsigned char guardByte) {
	unsigned char *guard = (unsigned char *)ptr + size;
	for (size_t i = 0; i < guardSize; i++) {
		if (guard[i] != guardByte) {
			return 0;
		}
	}
	return 1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

const sqlGetDataGuardSize = 4096
const sqlGetDataGuardCheckSize = 64
const sqlGetDataGuardByte = byte(0xa5)

var errSQLGetDataBufferOverflow = errors.New("ODBC driver wrote beyond the SQLGetData output buffer")
var errSQLGetDataIndicatorOverflow = errors.New("ODBC driver wrote beyond the SQLGetData length indicator")
var errSQLGetDataAllocation = errors.New("could not allocate native SQLGetData buffer")

type sqlGetDataCall func(SQLPOINTER, SQLLEN, *SQLLEN) SQLRETURN

// SQLGetDataBuffer keeps driver-owned output outside the Go heap. Some legacy
// drivers write beyond, or retain, SQLGetData pointers despite the ODBC
// contract; the guard regions turn small overruns into a query error instead
// of delayed Go heap corruption.
type SQLGetDataBuffer struct {
	data        unsafe.Pointer
	dataSize    int
	indicator   *SQLLEN
	allocations []unsafe.Pointer
}

func NewSQLGetDataBuffer() *SQLGetDataBuffer {
	return &SQLGetDataBuffer{}
}

func (b *SQLGetDataBuffer) GetData(statementHandle SQLHSTMT, colOrParamNum SQLUSMALLINT, targetType SQLSMALLINT, dst []byte, vallen *SQLLEN) (SQLRETURN, error) {
	defer lockNativeCall()()
	return b.getData(dst, vallen, func(targetValuePtr SQLPOINTER, bufferLength SQLLEN, nativeLen *SQLLEN) SQLRETURN {
		return nativeSQLGetData(statementHandle, colOrParamNum, targetType, targetValuePtr, bufferLength, nativeLen)
	})
}

func (b *SQLGetDataBuffer) getData(dst []byte, vallen *SQLLEN, call sqlGetDataCall) (SQLRETURN, error) {
	if err := b.ensureData(len(dst)); err != nil {
		return SQLRETURN(-1), err
	}
	if err := b.ensureIndicator(); err != nil {
		return SQLRETURN(-1), err
	}

	activeSize := len(dst)
	indicatorSize := int(unsafe.Sizeof(*b.indicator))
	C.initializeGuardedBuffer(b.data, C.size_t(activeSize), C.size_t(sqlGetDataGuardCheckSize), C.uchar(sqlGetDataGuardByte))
	C.initializeGuardedBuffer(unsafe.Pointer(b.indicator), C.size_t(indicatorSize), C.size_t(sqlGetDataGuardCheckSize), C.uchar(sqlGetDataGuardByte))
	*b.indicator = *vallen

	ret := call(SQLPOINTER(b.data), SQLLEN(len(dst)), b.indicator)
	*vallen = *b.indicator
	if len(dst) > 0 {
		C.memcpy(unsafe.Pointer(&dst[0]), b.data, C.size_t(len(dst)))
	}

	if C.guardedBufferIsValid(b.data, C.size_t(activeSize), C.size_t(sqlGetDataGuardCheckSize), C.uchar(sqlGetDataGuardByte)) == 0 {
		return ret, errSQLGetDataBufferOverflow
	}
	if C.guardedBufferIsValid(unsafe.Pointer(b.indicator), C.size_t(indicatorSize), C.size_t(sqlGetDataGuardCheckSize), C.uchar(sqlGetDataGuardByte)) == 0 {
		return ret, errSQLGetDataIndicatorOverflow
	}
	return ret, nil
}

func (b *SQLGetDataBuffer) ensureData(size int) error {
	if size <= b.dataSize && b.data != nil {
		return nil
	}
	allocation := C.malloc(C.size_t(size + sqlGetDataGuardSize))
	if allocation == nil {
		return errSQLGetDataAllocation
	}
	b.allocations = append(b.allocations, allocation)
	b.data = allocation
	b.dataSize = size
	return nil
}

func (b *SQLGetDataBuffer) ensureIndicator() error {
	if b.indicator != nil {
		return nil
	}
	size := int(unsafe.Sizeof(SQLLEN(0))) + sqlGetDataGuardSize
	allocation := C.malloc(C.size_t(size))
	if allocation == nil {
		return errSQLGetDataAllocation
	}
	b.allocations = append(b.allocations, allocation)
	b.indicator = (*SQLLEN)(allocation)
	return nil
}

func (b *SQLGetDataBuffer) Close() {
	for _, allocation := range b.allocations {
		C.free(allocation)
	}
	b.data = nil
	b.dataSize = 0
	b.indicator = nil
	b.allocations = nil
}
