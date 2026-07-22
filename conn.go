// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/alexbrainman/odbc/api"
)

type Conn struct {
	h                api.SQLHDBC
	tx               *Tx
	bad              bool
	isMSAccessDriver bool
	unicodeResults   bool
	unicodeCType     api.SQLSMALLINT
	sqlTextEncoding  string
	columnBinding    bool
	serializedLife   bool
}

var accessDriverSubstr = strings.ToUpper(strings.Replace("DRIVER={Microsoft Access Driver", " ", "", -1))
var connectMu sync.Mutex

func (d *Driver) Open(dsn string) (driver.Conn, error) {
	if d.initErr != nil {
		return nil, d.initErr
	}
	odbcDSN, unicodeResults, unicodeCType, sqlTextEncoding, columnBinding, connectTimeout := parseDriverOptions(dsn)
	if legacyDriver, ok := incompatibleInterSystemsDriver(odbcDSN); ok && runtime.GOOS != "windows" && unsafe.Sizeof(api.SQLLEN(0)) == 8 {
		return nil, fmt.Errorf("legacy InterSystems ODBC driver %q is incompatible with %d-byte SQLLEN; use the ur64 driver", legacyDriver, unsafe.Sizeof(api.SQLLEN(0)))
	}

	// Some legacy ODBC drivers mutate process-global state while connecting.
	// Serialize GraphNG connection lifecycle calls without changing behavior
	// for other consumers of this driver.
	serializedLife := strings.Contains(strings.ToLower(dsn), "graphng")
	if serializedLife {
		api.EnableNativeCallSerialization()
		connectMu.Lock()
		defer connectMu.Unlock()
	}

	var out api.SQLHANDLE
	ret := api.SQLAllocHandle(api.SQL_HANDLE_DBC, api.SQLHANDLE(d.h), &out)
	if IsError(ret) {
		return nil, NewError("SQLAllocHandle", d.h)
	}
	h := api.SQLHDBC(out)
	drv.Stats.updateHandleCount(api.SQL_HANDLE_DBC, 1)

	if connectTimeout > 0 {
		ret = api.SQLSetConnectUIntPtrAttr(h, api.SQL_ATTR_LOGIN_TIMEOUT, uintptr(connectTimeout), 0)
		if IsError(ret) {
			defer releaseHandle(h)
			return nil, NewError("SQLSetConnectAttr(SQL_ATTR_LOGIN_TIMEOUT)", h)
		}
	}
	b := api.StringToUTF16(odbcDSN)
	ret = api.SQLDriverConnect(h, 0,
		(*api.SQLWCHAR)(unsafe.Pointer(&b[0])), api.SQL_NTS,
		nil, 0, nil, api.SQL_DRIVER_NOPROMPT)
	runtime.KeepAlive(b)
	if IsError(ret) {
		defer releaseHandle(h)
		return nil, NewError("SQLDriverConnect", h)
	}
	isAccess := strings.Contains(strings.ToUpper(strings.Replace(odbcDSN, " ", "", -1)), accessDriverSubstr)
	return &Conn{
		h:                h,
		isMSAccessDriver: isAccess,
		unicodeResults:   unicodeResults,
		unicodeCType:     unicodeCType,
		sqlTextEncoding:  sqlTextEncoding,
		columnBinding:    columnBinding,
		serializedLife:   serializedLife,
	}, nil
}

func incompatibleInterSystemsDriver(dsn string) (string, bool) {
	for _, part := range strings.Split(dsn, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Driver") {
			continue
		}
		configuredDriver := strings.Trim(strings.TrimSpace(value), "{}")
		driverName := strings.ToLower(configuredDriver)
		if slash := strings.LastIndexAny(driverName, "/\\"); slash >= 0 {
			driverName = driverName[slash+1:]
		}
		switch driverName {
		case "cache", "cache35", "cacheu", "cacheu35", "cacheuw", "cacheuw35",
			"iris", "iris35", "irisu", "irisu35", "irisuw", "irisuw35",
			"libcacheodbc.so", "libcacheodbc35.so", "libcacheodbcu.so", "libcacheodbcu35.so", "libcacheodbcuw.so", "libcacheodbcuw35.so",
			"libirisodbc.so", "libirisodbc35.so", "libirisodbcu.so", "libirisodbcu35.so", "libirisodbcuw.so", "libirisodbcuw35.so":
			return configuredDriver, true
		}
	}
	return "", false
}

func parseDriverOptions(dsn string) (string, bool, api.SQLSMALLINT, string, bool, uint64) {
	parts := strings.Split(dsn, ";")
	kept := make([]string, 0, len(parts))
	unicodeResults := false
	unicodeCType := api.SQLSMALLINT(api.SQL_C_WCHAR)
	sqlTextEncoding := "wide"
	columnBinding := true
	columnBindingConfigured := false
	graphNGConfigured := false
	connectTimeout := uint64(0)
	connectTimeoutConfigured := false
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "GraphNGUnicodeResults") {
			graphNGConfigured = true
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "1", "true", "yes", "on":
				unicodeResults = true
			}
			continue
		}
		if ok && strings.EqualFold(strings.TrimSpace(key), "GraphNGUnicodeCType") {
			graphNGConfigured = true
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "char", "sql_c_char":
				unicodeCType = api.SQLSMALLINT(api.SQL_C_CHAR)
			case "wchar", "wide", "sql_c_wchar":
				unicodeCType = api.SQLSMALLINT(api.SQL_C_WCHAR)
			}
			continue
		}
		if ok && strings.EqualFold(strings.TrimSpace(key), "GraphNGSQLTextEncoding") {
			graphNGConfigured = true
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "utf8", "utf-8", "char", "ansi", "narrow":
				sqlTextEncoding = "utf8"
			case "wide", "wchar", "unicode", "utf16", "utf-16":
				sqlTextEncoding = "wide"
			}
			continue
		}
		if ok && strings.EqualFold(strings.TrimSpace(key), "GraphNGColumnBinding") {
			graphNGConfigured = true
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "1", "true", "yes", "on", "enabled":
				columnBinding = true
				columnBindingConfigured = true
			case "0", "false", "no", "off", "disabled":
				columnBinding = false
				columnBindingConfigured = true
			}
			continue
		}
		if ok && strings.EqualFold(strings.TrimSpace(key), "GraphNGConnectTimeout") {
			graphNGConfigured = true
			if seconds, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32); err == nil {
				connectTimeout = seconds
				connectTimeoutConfigured = true
			}
			continue
		}
		kept = append(kept, part)
	}
	if graphNGConfigured && !columnBindingConfigured {
		// SQLBindCol lets the native driver retain Go buffer pointers between
		// cgo calls. GraphNG uses SQLGetData for every result mode instead.
		columnBinding = false
	}
	if graphNGConfigured && !connectTimeoutConfigured {
		connectTimeout = 10
	}
	return strings.Join(kept, ";"), unicodeResults, unicodeCType, sqlTextEncoding, columnBinding, connectTimeout
}

func (c *Conn) Close() (err error) {
	if c.serializedLife {
		connectMu.Lock()
		defer connectMu.Unlock()
	}
	if c.tx != nil {
		c.tx.Rollback()
	}
	h := c.h
	defer func() {
		c.h = api.SQLHDBC(api.SQL_NULL_HDBC)
		e := releaseHandle(h)
		if err == nil {
			err = e
		}
	}()
	ret := api.SQLDisconnect(c.h)
	if IsError(ret) {
		return c.newError("SQLDisconnect", h)
	}
	return err
}

func (c *Conn) newError(apiName string, handle interface{}) error {
	err := NewError(apiName, handle)
	if err == driver.ErrBadConn {
		c.bad = true
	}
	return err
}

// QueryContext implements the driver.QueryerContext interface.
// As per the specifications, it honours the context timeout and returns when the context is cancelled.
// When the context is cancelled, it first cancels the statement, closes it, and then returns an error.
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	// prepare the statement
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}

	// check if context is canceled before executing the query
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if len(dargs) == 0 {
		return c.queryDirect(ctx, query)
	}

	os, err := c.PrepareODBCStmt(query)
	if err != nil {
		return nil, err
	}
	defer os.closeByStmt()

	// execute the statement
	rowsChan := make(chan driver.Rows)
	errorChan := make(chan error)
	go func() {
		err := c.wrapQuery(ctx, os, dargs)
		if err != nil {
			errorChan <- err
			return
		}
		os.usedByRows = true
		rowsChan <- &Rows{os: os}
	}()
	return c.waitQuery(ctx, os, rowsChan, errorChan)
}

func (c *Conn) queryDirect(ctx context.Context, query string) (driver.Rows, error) {
	os, err := c.NewODBCStmt()
	if err != nil {
		return nil, err
	}
	defer os.closeByStmt()

	rowsChan := make(chan driver.Rows)
	errorChan := make(chan error)
	go func() {
		if err := c.wrapQueryDirect(ctx, os, query); err != nil {
			errorChan <- err
			return
		}
		os.usedByRows = true
		rowsChan <- &Rows{os: os}
	}()

	select {
	case <-ctx.Done():
		os.Cancel()
		select {
		case <-errorChan:
			return nil, ctx.Err()
		case rows := <-rowsChan:
			return rows, nil
		}
	case err := <-errorChan:
		return nil, err
	case rows := <-rowsChan:
		return rows, nil
	}
}

func (c *Conn) wrapQueryDirect(ctx context.Context, os *ODBCStmt, query string) error {
	if err := os.ExecDirect(query, c); err != nil {
		return err
	}

	if err := os.BindColumns(); err != nil {
		return err
	}
	return nil
}

// wrapQuery is following the same logic as `stmt.Query()` except that we don't use a lock
// because the ODBC statement doesn't get exposed externally.
func (c *Conn) wrapQuery(ctx context.Context, os *ODBCStmt, dargs []driver.Value) error {
	if err := os.Exec(dargs, c); err != nil {
		return err
	}

	if err := os.BindColumns(); err != nil {
		return err
	}
	return nil
}

// waitQuery waits for either os rows or error to arrive from rowsChan and errorChan.
// waitQuery also waits for ctx to signal completion.
// The function returns received rows or the error.
func (c *Conn) waitQuery(ctx context.Context, os *ODBCStmt, rowsChan <-chan driver.Rows, errorChan <-chan error) (driver.Rows, error) {
	select {
	case <-ctx.Done():
		// context has been cancelled or has expired, cancel the statement and ignore the os.Cancel error
		os.Cancel()
		// the statement has been cancelled, the query execution should eventually succeed or fail now
		select {
		// ignore the ODBC error and return ctx.Err() instead
		case <-errorChan:
			return nil, ctx.Err()
		case rows := <-rowsChan:
			return rows, nil
		}
	case err := <-errorChan:
		return nil, err
	case rows := <-rowsChan:
		return rows, nil
	}
}

// namedValueToValue is a utility function that converts a driver.NamedValue into a driver.Value.
// Source:
// https://github.com/golang/go/blob/03ac39ce5e6af4c4bca58b54d5b160a154b7aa0e/src/database/sql/ctxutil.go#L137-L146
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	dargs := make([]driver.Value, len(named))
	for n, param := range named {
		if len(param.Name) > 0 {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		dargs[n] = param.Value
	}
	return dargs, nil
}
