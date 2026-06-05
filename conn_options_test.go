package odbc

import "testing"

func TestParseDriverOptionsRemovesUnicodeResults(t *testing.T) {
	dsn, unicodeResults := parseDriverOptions("Driver=Iris;Host=127.0.0.1;GraphNGUnicodeResults=true;UID=_system")
	if !unicodeResults {
		t.Fatal("unicodeResults = false, want true")
	}
	if dsn != "Driver=Iris;Host=127.0.0.1;UID=_system" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestParseDriverOptionsDefaultsUnicodeResultsOff(t *testing.T) {
	dsn, unicodeResults := parseDriverOptions("Driver=Iris;Host=127.0.0.1")
	if unicodeResults {
		t.Fatal("unicodeResults = true, want false")
	}
	if dsn != "Driver=Iris;Host=127.0.0.1" {
		t.Fatalf("dsn = %q", dsn)
	}
}
