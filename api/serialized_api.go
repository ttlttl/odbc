package api

import (
	"sync"
	"sync/atomic"
)

// Some legacy ODBC drivers keep process-wide mutable state and corrupt memory
// when different handles enter the driver concurrently.
var nativeCallMu sync.Mutex
var serializeNativeCalls atomic.Bool

// EnableNativeCallSerialization enables process-wide protection for legacy
// drivers. It is intentionally opt-in so unrelated ODBC users retain their
// existing concurrency behavior.
func EnableNativeCallSerialization() {
	serializeNativeCalls.Store(true)
}

func lockNativeCall() func() {
	if !serializeNativeCalls.Load() {
		return func() {}
	}
	nativeCallMu.Lock()
	return nativeCallMu.Unlock
}

func SQLAllocHandle(handleType SQLSMALLINT, inputHandle SQLHANDLE, outputHandle *SQLHANDLE) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLAllocHandle(handleType, inputHandle, outputHandle)
}

func SQLBindCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLBindCol(statementHandle, columnNumber, targetType, targetValuePtr, bufferLength, vallen)
}

func SQLBindParameter(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, inputOutputType SQLSMALLINT, valueType SQLSMALLINT, parameterType SQLSMALLINT, columnSize SQLULEN, decimalDigits SQLSMALLINT, parameterValue SQLPOINTER, bufferLength SQLLEN, ind *SQLLEN) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLBindParameter(statementHandle, parameterNumber, inputOutputType, valueType, parameterType, columnSize, decimalDigits, parameterValue, bufferLength, ind)
}

// SQLCancel must remain callable while an executing statement owns the lock.
func SQLCancel(statementHandle SQLHSTMT) SQLRETURN {
	return nativeSQLCancel(statementHandle)
}

func SQLCloseCursor(statementHandle SQLHSTMT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLCloseCursor(statementHandle)
}

func SQLDescribeCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, columnName *SQLWCHAR, bufferLength SQLSMALLINT, nameLengthPtr *SQLSMALLINT, dataTypePtr *SQLSMALLINT, columnSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLDescribeCol(statementHandle, columnNumber, columnName, bufferLength, nameLengthPtr, dataTypePtr, columnSizePtr, decimalDigitsPtr, nullablePtr)
}

func SQLDescribeParam(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, dataTypePtr *SQLSMALLINT, parameterSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLDescribeParam(statementHandle, parameterNumber, dataTypePtr, parameterSizePtr, decimalDigitsPtr, nullablePtr)
}

func SQLDisconnect(connectionHandle SQLHDBC) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLDisconnect(connectionHandle)
}

func SQLDriverConnect(connectionHandle SQLHDBC, windowHandle SQLHWND, inConnectionString *SQLWCHAR, stringLength1 SQLSMALLINT, outConnectionString *SQLWCHAR, bufferLength SQLSMALLINT, stringLength2Ptr *SQLSMALLINT, driverCompletion SQLUSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLDriverConnect(connectionHandle, windowHandle, inConnectionString, stringLength1, outConnectionString, bufferLength, stringLength2Ptr, driverCompletion)
}

func SQLEndTran(handleType SQLSMALLINT, handle SQLHANDLE, completionType SQLSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLEndTran(handleType, handle, completionType)
}

func SQLExecute(statementHandle SQLHSTMT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLExecute(statementHandle)
}

func SQLExecDirect(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLExecDirect(statementHandle, statementText, textLength)
}

func SQLExecDirectA(statementHandle SQLHSTMT, statementText *SQLCHAR, textLength SQLINTEGER) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLExecDirectA(statementHandle, statementText, textLength)
}

func SQLFetch(statementHandle SQLHSTMT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLFetch(statementHandle)
}

func SQLFreeHandle(handleType SQLSMALLINT, handle SQLHANDLE) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLFreeHandle(handleType, handle)
}

func SQLGetData(statementHandle SQLHSTMT, colOrParamNum SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLGetData(statementHandle, colOrParamNum, targetType, targetValuePtr, bufferLength, vallen)
}

func SQLGetDiagRec(handleType SQLSMALLINT, handle SQLHANDLE, recNumber SQLSMALLINT, sqlState *SQLWCHAR, nativeErrorPtr *SQLINTEGER, messageText *SQLWCHAR, bufferLength SQLSMALLINT, textLengthPtr *SQLSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLGetDiagRec(handleType, handle, recNumber, sqlState, nativeErrorPtr, messageText, bufferLength, textLengthPtr)
}

func SQLNumParams(statementHandle SQLHSTMT, parameterCountPtr *SQLSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLNumParams(statementHandle, parameterCountPtr)
}

func SQLMoreResults(statementHandle SQLHSTMT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLMoreResults(statementHandle)
}

func SQLNumResultCols(statementHandle SQLHSTMT, columnCountPtr *SQLSMALLINT) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLNumResultCols(statementHandle, columnCountPtr)
}

func SQLPrepare(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLPrepare(statementHandle, statementText, textLength)
}

func SQLPrepareA(statementHandle SQLHSTMT, statementText *SQLCHAR, textLength SQLINTEGER) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLPrepareA(statementHandle, statementText, textLength)
}

func SQLRowCount(statementHandle SQLHSTMT, rowCountPtr *SQLLEN) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLRowCount(statementHandle, rowCountPtr)
}

func SQLSetEnvAttr(environmentHandle SQLHENV, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLSetEnvAttr(environmentHandle, attribute, valuePtr, stringLength)
}

func SQLSetConnectAttr(connectionHandle SQLHDBC, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) SQLRETURN {
	defer lockNativeCall()()
	return nativeSQLSetConnectAttr(connectionHandle, attribute, valuePtr, stringLength)
}
