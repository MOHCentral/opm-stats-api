package tests

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStressEvents simulates high load on the ingestion endpoint
// Run with: ./cli test stress
func TestStressEvents(t *testing.T) {
	// Skip if strictly running unit tests or short mode
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	totalEvents := 1000
	concurrency := 20
	
	t.Logf("=== Starting Stress Test (%d events, %d concurrent) ===", totalEvents, concurrency)

	var successCount int64
	var failCount int64

	start := time.Now()
	
	var wg sync.WaitGroup
	jobs := make(chan int, totalEvents)

	// Worker pool
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				params := map[string]string{
					"attacker_name": fmt.Sprintf("StressUser_%d", i%50),
					"attacker_guid": fmt.Sprintf("STRESS-GUID-%d", i%50),
					"victim_name":   fmt.Sprintf("StressVic_%d", rand.Intn(50)),
					"victim_guid":   fmt.Sprintf("STRESS-GUID-%d", rand.Intn(50)),
					"weapon":        "M1 Garand",
					"hitloc":        "head",
					"stress_id":     fmt.Sprintf("%d", i),
				}
				
				// Re-use sendEvent helper (assuming it returns error or we panic on fail inside)
				// Modify sendEvent in e2e_test.go to return error instead of Fatalf if we want to count failures
				// For now, we will construct a local sender to avoid failing the whole test on one timeout
				
				if err := sendEventSafe(t, "kill", params); err == nil {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}()
	}

	// Queue jobs
	for i := 0; i < totalEvents; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	duration := time.Since(start)
	rate := float64(totalEvents) / duration.Seconds()

	t.Logf("=== Stress Test Complete ===")
	t.Logf("Total: %d | Success: %d | Failed: %d", totalEvents, successCount, failCount)
	t.Logf("Duration: %v | Rate: %.2f events/sec", duration, rate)

	if failCount > 0 {
		t.Logf("WARNING: %d events failed to ingest", failCount)
		// We don't fail the test unless failure rate is high, as load shedding is expected
		if float64(failCount)/float64(totalEvents) > 0.1 {
			t.Errorf("Failure rate too high: %.2f%%", float64(failCount)/float64(totalEvents)*100)
		}
	}
}


