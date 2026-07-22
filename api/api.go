// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

//go:generate go run golang.org/x/sys/windows/mkwinsyscall@v0.20.0 -output zapi_windows.go api.go

//go:generate sh -c "./mksyscall_unix.pl api.go | gofmt > zapi_unix.go"

import (
	"unicode/utf16"
)

type (
	SQL_DATE_STRUCT struct {
		Year  SQLSMALLINT
		Month SQLUSMALLINT
		Day   SQLUSMALLINT
	}

	SQL_TIME_STRUCT struct {
		Hour   SQLUSMALLINT
		Minute SQLUSMALLINT
		Second SQLUSMALLINT
	}

	SQL_SS_TIME2_STRUCT struct {
		Hour     SQLUSMALLINT
		Minute   SQLUSMALLINT
		Second   SQLUSMALLINT
		Fraction SQLUINTEGER
	}

	SQL_TIMESTAMP_STRUCT struct {
		Year     SQLSMALLINT
		Month    SQLUSMALLINT
		Day      SQLUSMALLINT
		Hour     SQLUSMALLINT
		Minute   SQLUSMALLINT
		Second   SQLUSMALLINT
		Fraction SQLUINTEGER
	}
)

// The generated functions are deliberately private. Public wrappers in
// serialized_api.go protect drivers that are not safe for concurrent calls.
//sys	nativeSQLAllocHandle(handleType SQLSMALLINT, inputHandle SQLHANDLE, outputHandle *SQLHANDLE) (ret SQLRETURN) = odbc32.SQLAllocHandle
//sys	nativeSQLBindCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) (ret SQLRETURN) = odbc32.SQLBindCol
//sys	nativeSQLBindParameter(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, inputOutputType SQLSMALLINT, valueType SQLSMALLINT, parameterType SQLSMALLINT, columnSize SQLULEN, decimalDigits SQLSMALLINT, parameterValue SQLPOINTER, bufferLength SQLLEN, ind *SQLLEN) (ret SQLRETURN) = odbc32.SQLBindParameter
//sys	nativeSQLCloseCursor(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLCloseCursor
//sys	nativeSQLDescribeCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, columnName *SQLWCHAR, bufferLength SQLSMALLINT, nameLengthPtr *SQLSMALLINT, dataTypePtr *SQLSMALLINT, columnSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLDescribeColW
//sys	nativeSQLDescribeParam(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, dataTypePtr *SQLSMALLINT, parameterSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLDescribeParam
//sys	nativeSQLDisconnect(connectionHandle SQLHDBC) (ret SQLRETURN) = odbc32.SQLDisconnect
//sys	nativeSQLDriverConnect(connectionHandle SQLHDBC, windowHandle SQLHWND, inConnectionString *SQLWCHAR, stringLength1 SQLSMALLINT, outConnectionString *SQLWCHAR, bufferLength SQLSMALLINT, stringLength2Ptr *SQLSMALLINT, driverCompletion SQLUSMALLINT) (ret SQLRETURN) = odbc32.SQLDriverConnectW
//sys	nativeSQLEndTran(handleType SQLSMALLINT, handle SQLHANDLE, completionType SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLEndTran
//sys	nativeSQLExecute(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLExecute
//sys	nativeSQLExecDirect(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLExecDirectW
//sys	nativeSQLExecDirectA(statementHandle SQLHSTMT, statementText *SQLCHAR, textLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLExecDirect
//sys	nativeSQLFetch(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLFetch
//sys	nativeSQLFreeHandle(handleType SQLSMALLINT, handle SQLHANDLE) (ret SQLRETURN) = odbc32.SQLFreeHandle
//sys	nativeSQLGetData(statementHandle SQLHSTMT, colOrParamNum SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) (ret SQLRETURN) = odbc32.SQLGetData
//sys	nativeSQLGetDiagRec(handleType SQLSMALLINT, handle SQLHANDLE, recNumber SQLSMALLINT, sqlState *SQLWCHAR, nativeErrorPtr *SQLINTEGER, messageText *SQLWCHAR, bufferLength SQLSMALLINT, textLengthPtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLGetDiagRecW
//sys	nativeSQLNumParams(statementHandle SQLHSTMT, parameterCountPtr *SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLNumParams
//sys	nativeSQLMoreResults(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLMoreResults
//sys	nativeSQLNumResultCols(statementHandle SQLHSTMT, columnCountPtr *SQLSMALLINT)  (ret SQLRETURN) = odbc32.SQLNumResultCols
//sys	nativeSQLPrepare(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLPrepareW
//sys	nativeSQLPrepareA(statementHandle SQLHSTMT, statementText *SQLCHAR, textLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLPrepare
//sys	nativeSQLRowCount(statementHandle SQLHSTMT, rowCountPtr *SQLLEN) (ret SQLRETURN) = odbc32.SQLRowCount
//sys	nativeSQLSetEnvAttr(environmentHandle SQLHENV, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLSetEnvAttr
//sys	nativeSQLSetConnectAttr(connectionHandle SQLHDBC, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) = odbc32.SQLSetConnectAttrW
//sys	nativeSQLCancel(statementHandle SQLHSTMT) (ret SQLRETURN) = odbc32.SQLCancel

// UTF16ToString returns the UTF-8 encoding of the UTF-16 sequence s,
// with a terminating NUL removed.
func UTF16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
	}
	return string(utf16.Decode(s))
}

// StringToUTF16 returns the UTF-16 encoding of the UTF-8 string s,
// with a terminating NUL added.
func StringToUTF16(s string) []uint16 { return utf16.Encode([]rune(s + "\x00")) }

// StringToUTF16Ptr returns pointer to the UTF-16 encoding of
// the UTF-8 string s, with a terminating NUL added.
func StringToUTF16Ptr(s string) *uint16 { return &StringToUTF16(s)[0] }
