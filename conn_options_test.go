package odbc

import (
	"testing"

	"github.com/alexbrainman/odbc/api"
)

func TestParseDriverOptionsRemovesUnicodeResults(t *testing.T) {
	dsn, unicodeResults, unicodeCType, sqlTextEncoding, columnBinding, connectTimeout := parseDriverOptions("Driver=Iris;Host=127.0.0.1;GraphNGUnicodeResults=true;UID=_system")
	if !unicodeResults {
		t.Fatal("unicodeResults = false, want true")
	}
	if unicodeCType != api.SQL_C_WCHAR {
		t.Fatalf("unicodeCType = %d, want SQL_C_WCHAR", unicodeCType)
	}
	if sqlTextEncoding != "wide" {
		t.Fatalf("sqlTextEncoding = %q, want wide", sqlTextEncoding)
	}
	if columnBinding {
		t.Fatal("columnBinding = true, want false for GraphNG unicode results")
	}
	if dsn != "Driver=Iris;Host=127.0.0.1;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}
	if connectTimeout != 10 {
		t.Fatalf("connectTimeout = %d, want 10", connectTimeout)
	}
}

func TestParseDriverOptionsDefaultsUnicodeResultsOff(t *testing.T) {
	dsn, unicodeResults, unicodeCType, sqlTextEncoding, columnBinding, connectTimeout := parseDriverOptions("Driver=Iris;Host=127.0.0.1")
	if unicodeResults {
		t.Fatal("unicodeResults = true, want false")
	}
	if unicodeCType != api.SQL_C_WCHAR {
		t.Fatalf("unicodeCType = %d, want SQL_C_WCHAR", unicodeCType)
	}
	if sqlTextEncoding != "wide" {
		t.Fatalf("sqlTextEncoding = %q, want wide", sqlTextEncoding)
	}
	if !columnBinding {
		t.Fatal("columnBinding = false, want true without GraphNG unicode results")
	}
	if dsn != "Driver=Iris;Host=127.0.0.1" {
		t.Fatalf("dsn = %q", dsn)
	}
	if connectTimeout != 0 {
		t.Fatalf("connectTimeout = %d, want 0", connectTimeout)
	}
}

func TestParseDriverOptionsReadsUnicodeCType(t *testing.T) {
	dsn, unicodeResults, unicodeCType, sqlTextEncoding, columnBinding, _ := parseDriverOptions("Driver=Cacheu35;GraphNGUnicodeResults=true;GraphNGUnicodeCType=char;UID=_system")
	if !unicodeResults {
		t.Fatal("unicodeResults = false, want true")
	}
	if unicodeCType != api.SQL_C_CHAR {
		t.Fatalf("unicodeCType = %d, want SQL_C_CHAR", unicodeCType)
	}
	if sqlTextEncoding != "wide" {
		t.Fatalf("sqlTextEncoding = %q, want wide", sqlTextEncoding)
	}
	if columnBinding {
		t.Fatal("columnBinding = true, want false for Cache unicode results")
	}
	if dsn != "Driver=Cacheu35;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestParseDriverOptionsReadsSQLTextEncoding(t *testing.T) {
	dsn, _, _, sqlTextEncoding, _, _ := parseDriverOptions("Driver=Iris;GraphNGSQLTextEncoding=utf8;UID=_system")
	if sqlTextEncoding != "utf8" {
		t.Fatalf("sqlTextEncoding = %q, want utf8", sqlTextEncoding)
	}
	if dsn != "Driver=Iris;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestParseDriverOptionsReadsColumnBindingOverride(t *testing.T) {
	dsn, unicodeResults, _, _, columnBinding, _ := parseDriverOptions("Driver=Cacheu35;GraphNGUnicodeResults=true;GraphNGColumnBinding=enabled;UID=_system")
	if !unicodeResults {
		t.Fatal("unicodeResults = false, want true")
	}
	if !columnBinding {
		t.Fatal("columnBinding = false, want explicit override true")
	}
	if dsn != "Driver=Cacheu35;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}

	_, _, _, _, columnBinding, _ = parseDriverOptions("Driver=Cacheu35;GraphNGColumnBinding=disabled;UID=_system")
	if columnBinding {
		t.Fatal("columnBinding = true, want explicit override false")
	}
}

func TestParseDriverOptionsDisablesBindingForGraphNGNonUnicodeResults(t *testing.T) {
	dsn, unicodeResults, _, _, columnBinding, connectTimeout := parseDriverOptions("Driver=IRIS;GraphNGUnicodeResults=false;GraphNGConnectTimeout=10;UID=reader")
	if dsn != "Driver=IRIS;UID=reader" {
		t.Fatalf("dsn = %q", dsn)
	}
	if unicodeResults {
		t.Fatal("unicodeResults = true, want false")
	}
	if columnBinding {
		t.Fatal("columnBinding = true, want false for every GraphNG result mode")
	}
	if connectTimeout != 10 {
		t.Fatalf("connectTimeout = %d, want 10", connectTimeout)
	}
}

func TestParseDriverOptionsAllowsDisablingGraphNGConnectTimeout(t *testing.T) {
	_, _, _, _, _, connectTimeout := parseDriverOptions("Driver=IRIS;GraphNGUnicodeResults=false;GraphNGConnectTimeout=0")
	if connectTimeout != 0 {
		t.Fatalf("connectTimeout = %d, want 0", connectTimeout)
	}
}
