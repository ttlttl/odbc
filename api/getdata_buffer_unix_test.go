//go:build (darwin || linux || freebsd) && cgo
// +build darwin linux freebsd
// +build cgo

package api

import (
	"bytes"
	"testing"
	"unsafe"
)

func TestSQLGetDataBufferCopiesNativeOutput(t *testing.T) {
	buffer := NewSQLGetDataBuffer()
	defer buffer.Close()
	dst := make([]byte, 8)
	var length SQLLEN

	ret, err := buffer.getData(dst, &length, func(ptr SQLPOINTER, size SQLLEN, indicator *SQLLEN) SQLRETURN {
		copy((*[1 << 20]byte)(ptr)[:int(size):int(size)], []byte("cache"))
		*indicator = 5
		return SQL_SUCCESS
	})
	if err != nil {
		t.Fatal(err)
	}
	if ret != SQL_SUCCESS || length != 5 || !bytes.Equal(dst[:5], []byte("cache")) {
		t.Fatalf("GetData() = ret %d, length %d, data %q", ret, length, dst)
	}
}

func TestSQLGetDataBufferDetectsOutputOverflow(t *testing.T) {
	buffer := NewSQLGetDataBuffer()
	defer buffer.Close()
	dst := make([]byte, 8)
	var length SQLLEN

	_, err := buffer.getData(dst, &length, func(ptr SQLPOINTER, size SQLLEN, indicator *SQLLEN) SQLRETURN {
		(*[1 << 20]byte)(ptr)[int(size)] = 0
		return SQL_SUCCESS
	})
	if err != errSQLGetDataBufferOverflow {
		t.Fatalf("GetData() error = %v, want output overflow", err)
	}
}

func TestSQLGetDataBufferDetectsIndicatorOverflow(t *testing.T) {
	buffer := NewSQLGetDataBuffer()
	defer buffer.Close()
	dst := make([]byte, 8)
	var length SQLLEN

	_, err := buffer.getData(dst, &length, func(ptr SQLPOINTER, size SQLLEN, indicator *SQLLEN) SQLRETURN {
		indicatorBytes := (*[1 << 20]byte)(unsafe.Pointer(indicator))[:int(unsafe.Sizeof(*indicator))+1]
		indicatorBytes[len(indicatorBytes)-1] = 0
		return SQL_SUCCESS
	})
	if err != errSQLGetDataIndicatorOverflow {
		t.Fatalf("GetData() error = %v, want indicator overflow", err)
	}
}
