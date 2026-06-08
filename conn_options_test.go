package odbc

import (
	"testing"

	"github.com/alexbrainman/odbc/api"
)

func TestParseDriverOptionsRemovesUnicodeResults(t *testing.T) {
	dsn, unicodeResults, unicodeCType := parseDriverOptions("Driver=Iris;Host=127.0.0.1;GraphNGUnicodeResults=true;UID=_system")
	if !unicodeResults {
		t.Fatal("unicodeResults = false, want true")
	}
	if unicodeCType != api.SQL_C_WCHAR {
		t.Fatalf("unicodeCType = %d, want SQL_C_WCHAR", unicodeCType)
	}
	if dsn != "Driver=Iris;Host=127.0.0.1;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestParseDriverOptionsDefaultsUnicodeResultsOff(t *testing.T) {
	dsn, unicodeResults, unicodeCType := parseDriverOptions("Driver=Iris;Host=127.0.0.1")
	if unicodeResults {
		t.Fatal("unicodeResults = true, want false")
	}
	if unicodeCType != api.SQL_C_WCHAR {
		t.Fatalf("unicodeCType = %d, want SQL_C_WCHAR", unicodeCType)
	}
	if dsn != "Driver=Iris;Host=127.0.0.1" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestParseDriverOptionsReadsUnicodeCType(t *testing.T) {
	dsn, unicodeResults, unicodeCType := parseDriverOptions("Driver=Cacheu35;GraphNGUnicodeResults=true;GraphNGUnicodeCType=char;UID=_system")
	if !unicodeResults {
		t.Fatal("unicodeResults = false, want true")
	}
	if unicodeCType != api.SQL_C_CHAR {
		t.Fatalf("unicodeCType = %d, want SQL_C_CHAR", unicodeCType)
	}
	if dsn != "Driver=Cacheu35;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}
}
