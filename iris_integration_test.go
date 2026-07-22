package odbc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIRISConcurrentConnectAndQuery(t *testing.T) {
	dsn := os.Getenv("ODBC_IRIS_TEST_DSN")
	if dsn == "" {
		t.Skip("ODBC_IRIS_TEST_DSN is not set")
	}

	duration := integrationDuration(t, "ODBC_IRIS_TEST_DURATION", 30*time.Second)
	workers := integrationWorkers(t, "ODBC_IRIS_TEST_WORKERS", 8)
	query := os.Getenv("ODBC_IRIS_TEST_QUERY")
	if query == "" {
		query = "SELECT 1"
	}
	minimumResultBytes := integrationWorkers(t, "ODBC_IRIS_TEST_MIN_RESULT_BYTES", 1)
	expectedValue := os.Getenv("ODBC_IRIS_TEST_EXPECTED_VALUE")
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	dsns := []string{dsn}
	if secondDSN := os.Getenv("ODBC_IRIS_SECOND_TEST_DSN"); secondDSN != "" {
		dsns = append(dsns, secondDSN)
	}
	dbs := make([]*sql.DB, 0, len(dsns))
	for _, testDSN := range dsns {
		db, err := sql.Open("odbc", testDSN)
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(workers)
		db.SetMaxIdleConns(workers)
		dbs = append(dbs, db)
		defer db.Close()
	}

	for _, db := range dbs {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := db.PingContext(pingCtx)
		pingCancel()
		if err != nil {
			t.Fatalf("initial ping failed: %v", err)
		}
	}

	var operations atomic.Int64
	errorsCh := make(chan error, workers+len(dsns))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		db := dbs[i%len(dbs)]
		wg.Add(1)
		go func(db *sql.DB) {
			defer wg.Done()
			for ctx.Err() == nil {
				var value string
				err := db.QueryRowContext(ctx, query).Scan(&value)
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				if len(value) < minimumResultBytes {
					reportIntegrationError(ctx, errorsCh, errors.New("query returned a shorter value than expected"))
					return
				}
				if expectedValue != "" && value != expectedValue {
					reportIntegrationError(ctx, errorsCh, fmt.Errorf("query value = %q (% x), want %q", value, []byte(value), expectedValue))
					return
				}
				operations.Add(1)
			}
		}(db)
	}

	for _, testDSN := range dsns {
		testDSN := testDSN
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				churnDB, err := sql.Open("odbc", testDSN)
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				churnDB.SetMaxOpenConns(1)
				churnDB.SetMaxIdleConns(0)
				err = churnDB.PingContext(ctx)
				closeErr := churnDB.Close()
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				if closeErr != nil {
					reportIntegrationError(ctx, errorsCh, closeErr)
					return
				}
				operations.Add(1)
			}
		}()
	}

	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
	if operations.Load() == 0 {
		t.Fatal("stress test completed without a successful operation")
	}
	t.Logf("completed %d operations in %s with %d query workers", operations.Load(), duration, workers)
}

func TestIRISRejectsLegacySQLLenDriver(t *testing.T) {
	dsn := os.Getenv("ODBC_IRIS_LEGACY_TEST_DSN")
	if dsn == "" {
		t.Skip("ODBC_IRIS_LEGACY_TEST_DSN is not set")
	}
	db, err := sql.Open("odbc", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = db.Ping()
	if err == nil || !strings.Contains(err.Error(), "8-byte SQLLEN") || !strings.Contains(err.Error(), "ur64") {
		t.Fatalf("Ping() error = %v, want SQLLEN width and ur64 guidance", err)
	}
	if expectedDriver := os.Getenv("ODBC_IRIS_LEGACY_EXPECTED_DRIVER"); expectedDriver != "" && !strings.Contains(err.Error(), expectedDriver) {
		t.Fatalf("Ping() error = %v, want driver name %q", err, expectedDriver)
	}
}

func TestIRISConcurrentMultiColumnQuery(t *testing.T) {
	dsn := os.Getenv("ODBC_IRIS_TEST_DSN")
	query := os.Getenv("ODBC_IRIS_MULTI_COLUMN_QUERY")
	if dsn == "" || query == "" {
		t.Skip("ODBC_IRIS_TEST_DSN and ODBC_IRIS_MULTI_COLUMN_QUERY are required")
	}

	duration := integrationDuration(t, "ODBC_IRIS_TEST_DURATION", 30*time.Second)
	workers := integrationWorkers(t, "ODBC_IRIS_TEST_WORKERS", 8)
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	dsns := []string{dsn}
	if secondDSN := os.Getenv("ODBC_IRIS_SECOND_TEST_DSN"); secondDSN != "" {
		dsns = append(dsns, secondDSN)
	}
	dbs := make([]*sql.DB, 0, len(dsns))
	for _, testDSN := range dsns {
		db, err := sql.Open("odbc", testDSN)
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(workers)
		db.SetMaxIdleConns(workers)
		dbs = append(dbs, db)
		defer db.Close()
	}

	var operations atomic.Int64
	errorsCh := make(chan error, workers+len(dsns))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		db := dbs[i%len(dbs)]
		wg.Add(1)
		go func(db *sql.DB) {
			defer wg.Done()
			for ctx.Err() == nil {
				rows, err := db.QueryContext(ctx, query)
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				columns, err := rows.Columns()
				if err != nil {
					rows.Close()
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				values := make([]interface{}, len(columns))
				dest := make([]interface{}, len(columns))
				for i := range values {
					dest[i] = &values[i]
				}
				for rows.Next() {
					if err := rows.Scan(dest...); err != nil {
						rows.Close()
						reportIntegrationError(ctx, errorsCh, err)
						return
					}
					operations.Add(1)
				}
				err = rows.Err()
				closeErr := rows.Close()
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				if closeErr != nil {
					reportIntegrationError(ctx, errorsCh, closeErr)
					return
				}
			}
		}(db)
	}

	// Exercise connection lifecycle calls while other handles fetch rows.
	for _, testDSN := range dsns {
		testDSN := testDSN
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				churnDB, err := sql.Open("odbc", testDSN)
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				churnDB.SetMaxOpenConns(1)
				churnDB.SetMaxIdleConns(0)
				err = churnDB.PingContext(ctx)
				closeErr := churnDB.Close()
				if err != nil {
					reportIntegrationError(ctx, errorsCh, err)
					return
				}
				if closeErr != nil {
					reportIntegrationError(ctx, errorsCh, closeErr)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
	if operations.Load() == 0 {
		t.Fatal("multi-column stress test completed without reading a row")
	}
	t.Logf("read %d multi-column rows in %s with %d workers", operations.Load(), duration, workers)
}

func reportIntegrationError(ctx context.Context, errorsCh chan<- error, err error) {
	if ctx.Err() != nil {
		return
	}
	errorsCh <- err
}

func integrationDuration(t *testing.T, name string, fallback time.Duration) time.Duration {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		t.Fatalf("invalid %s %q", name, value)
	}
	return duration
}

func integrationWorkers(t *testing.T, name string, fallback int) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	workers, err := strconv.Atoi(value)
	if err != nil || workers <= 0 {
		t.Fatalf("invalid %s %q", name, value)
	}
	return workers
}
