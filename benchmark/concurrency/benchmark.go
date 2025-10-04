package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type SessionResponse struct {
	SessionID string `json:"session_id"`
}

type CommitResponse struct {
	TxHash      string `json:"tx_hash"`
	BlockHeight int64  `json:"block_height"`
}

type WorkflowResult struct {
	Success  bool
	Latency  time.Duration
	ErrorMsg string
}

type Result struct {
	TotalRequests  int64
	SuccessfulReqs int64
	FailedReqs     int64
	Duration       time.Duration
	TPS            float64
	AvgLatency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
}

func main() {
	l1Nodes := flag.Int("l1", 4, "Number of L1 nodes")
	l2Nodes := flag.Int("l2", 2, "Number of L2 nodes")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	duration := flag.Int("duration", 30, "Test duration in seconds")
	l2Port := flag.String("port", "7000", "L2 port")
	packageID := flag.String("pkg", "PKG-001", "Package ID to use")
	flag.Parse()

	recordsDir := "./records"
	os.MkdirAll(recordsDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(recordsDir, fmt.Sprintf(
		"concurrency_%s_w%d_d%ds_l1-%d_l2-%d.csv",
		timestamp, *workers, *duration, *l1Nodes, *l2Nodes,
	))

	fmt.Println("========================================")
	fmt.Println("   CONCURRENCY BENCHMARK")
	fmt.Println("========================================")
	fmt.Printf("L1 Nodes:   %d\n", *l1Nodes)
	fmt.Printf("L2 Nodes:   %d\n", *l2Nodes)
	fmt.Printf("Workers:    %d\n", *workers)
	fmt.Printf("Duration:   %ds\n", *duration)
	fmt.Printf("L2 URL:     http://127.0.0.1:%s\n", *l2Port)
	fmt.Printf("Package ID: %s\n", *packageID)
	fmt.Printf("Output:     %s\n", filename)
	fmt.Println("========================================")
	fmt.Println("")

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", *l2Port)

	// Channels for communication
	stopChan := make(chan struct{})
	resultsChan := make(chan WorkflowResult, *workers*10)

	// Counters
	var totalReqs int64
	var successReqs int64
	var failedReqs int64
	var totalLatency int64
	var minLatency int64 = 1<<63 - 1
	var maxLatency int64 = 0

	// WaitGroup for workers
	var wg sync.WaitGroup

	// Start worker goroutines
	fmt.Println("Starting workers...")
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(i, baseURL, *packageID, stopChan, resultsChan, &wg)
	}

	// Start result collector
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for result := range resultsChan {
			atomic.AddInt64(&totalReqs, 1)

			if result.Success {
				atomic.AddInt64(&successReqs, 1)
				latencyNs := result.Latency.Nanoseconds()
				atomic.AddInt64(&totalLatency, latencyNs)

				// Update min latency
				for {
					old := atomic.LoadInt64(&minLatency)
					if latencyNs >= old || atomic.CompareAndSwapInt64(&minLatency, old, latencyNs) {
						break
					}
				}

				// Update max latency
				for {
					old := atomic.LoadInt64(&maxLatency)
					if latencyNs <= old || atomic.CompareAndSwapInt64(&maxLatency, old, latencyNs) {
						break
					}
				}
			} else {
				atomic.AddInt64(&failedReqs, 1)
			}

			// Progress indicator
			if totalReqs%10 == 0 {
				fmt.Printf("\rRequests: %d | Success: %d | Failed: %d | TPS: %.2f",
					totalReqs, successReqs, failedReqs,
					float64(totalReqs)/time.Since(time.Now().Add(-time.Duration(totalReqs)*time.Millisecond)).Seconds())
			}
		}
	}()

	// Run for specified duration
	startTime := time.Now()
	fmt.Printf("Running benchmark for %d seconds...\n", *duration)
	time.Sleep(time.Duration(*duration) * time.Second)

	// Stop workers
	close(stopChan)
	wg.Wait()
	close(resultsChan)
	collectorWg.Wait()

	elapsed := time.Since(startTime)

	// Calculate results
	tps := float64(totalReqs) / elapsed.Seconds()
	avgLatency := time.Duration(0)
	if successReqs > 0 {
		avgLatency = time.Duration(totalLatency / successReqs)
	}

	// Print results
	fmt.Println("\n\n========================================")
	fmt.Println("   BENCHMARK RESULTS")
	fmt.Println("========================================")
	fmt.Printf("Total Requests:    %d\n", totalReqs)
	fmt.Printf("Successful:        %d (%.2f%%)\n", successReqs, float64(successReqs)/float64(totalReqs)*100)
	fmt.Printf("Failed:            %d (%.2f%%)\n", failedReqs, float64(failedReqs)/float64(totalReqs)*100)
	fmt.Printf("Duration:          %v\n", elapsed)
	fmt.Printf("Throughput (TPS):  %.2f\n", tps)
	fmt.Printf("Avg Latency:       %v\n", avgLatency)
	fmt.Printf("Min Latency:       %v\n", time.Duration(minLatency))
	fmt.Printf("Max Latency:       %v\n", time.Duration(maxLatency))
	fmt.Println("========================================")

	// Save to CSV
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{
		"L1_Nodes", "L2_Nodes", "Workers", "Duration_s",
		"Total_Requests", "Successful", "Failed",
		"TPS", "Avg_Latency_ms", "Min_Latency_ms", "Max_Latency_ms",
	})

	writer.Write([]string{
		fmt.Sprintf("%d", *l1Nodes),
		fmt.Sprintf("%d", *l2Nodes),
		fmt.Sprintf("%d", *workers),
		fmt.Sprintf("%d", *duration),
		fmt.Sprintf("%d", totalReqs),
		fmt.Sprintf("%d", successReqs),
		fmt.Sprintf("%d", failedReqs),
		fmt.Sprintf("%.2f", tps),
		fmt.Sprintf("%.2f", float64(avgLatency.Milliseconds())),
		fmt.Sprintf("%.2f", float64(time.Duration(minLatency).Milliseconds())),
		fmt.Sprintf("%.2f", float64(time.Duration(maxLatency).Milliseconds())),
	})

	fmt.Printf("\nResults saved to: %s\n", filename)
}

func worker(id int, baseURL, packageID string, stopChan chan struct{}, resultsChan chan WorkflowResult, wg *sync.WaitGroup) {
	defer wg.Done()

	client := NewHTTPClient(baseURL)

	for {
		select {
		case <-stopChan:
			return
		default:
			start := time.Now()
			err := runWorkflow(client, packageID)
			latency := time.Since(start)

			result := WorkflowResult{
				Success: err == nil,
				Latency: latency,
			}
			if err != nil {
				result.ErrorMsg = err.Error()
			}

			resultsChan <- result
		}
	}
}

func runWorkflow(client *HTTPClient, packageID string) error {
	// 1. Start Session
	resp, err := client.POST("/session/start", map[string]interface{}{
		"operator_id": "OPR-001",
	})
	if err != nil {
		return fmt.Errorf("start session: %v", err)
	}
	var sessResp SessionResponse
	if err := UnmarshalBody(resp, &sessResp); err != nil {
		return fmt.Errorf("start session unmarshal: %v", err)
	}
	sessionID := sessResp.SessionID

	// 2. Scan Package
	endpoint := fmt.Sprintf("/session/%s/scan", sessionID)
	if _, err := client.GET(endpoint); err != nil {
		return fmt.Errorf("scan package: %v", err)
	}

	// 3. Validate Package
	endpoint = fmt.Sprintf("/session/%s/validate", sessionID)
	if _, err := client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
		"signature":  "sig_test_001",
	}); err != nil {
		return fmt.Errorf("validate package: %v", err)
	}

	// 4. Quality Check
	endpoint = fmt.Sprintf("/session/%s/qc", sessionID)
	if _, err := client.POST(endpoint, map[string]interface{}{
		"passed": true,
		"issues": []string{},
	}); err != nil {
		return fmt.Errorf("quality check: %v", err)
	}

	// 5. Label Package
	endpoint = fmt.Sprintf("/session/%s/label", sessionID)
	if _, err := client.POST(endpoint, map[string]interface{}{
		"courier_id": "CUR-001",
	}); err != nil {
		return fmt.Errorf("label package: %v", err)
	}

	// 6. Commit Session
	endpoint = fmt.Sprintf("/session/%s/commit", sessionID)
	if _, err := client.POST(endpoint, nil); err != nil {
		return fmt.Errorf("commit session: %v", err)
	}

	return nil
}
