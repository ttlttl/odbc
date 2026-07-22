package odbc

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
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
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	db, err := sql.Open("odbc", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(workers)
	db.SetMaxIdleConns(workers)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = db.PingContext(pingCtx)
	pingCancel()
	if err != nil {
		t.Fatalf("initial ping failed: %v", err)
	}

	var operations atomic.Int64
	errorsCh := make(chan error, workers+2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
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
				operations.Add(1)
			}
		}()
	}

	// Repeated short-lived pools force SQLDriverConnect/SQLDisconnect to run
	// while the persistent pool is executing and fetching rows.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			churnDB, err := sql.Open("odbc", dsn)
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
