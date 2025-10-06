package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type SessionResponse struct {
	SessionID string `json:"session_id"`
}

type CommitResponse struct {
	TxHash      string `json:"tx_hash"`
	BlockHeight int64  `json:"block_height"`
}

type Result struct {
	Step        string
	Latency     time.Duration
	BlockHeight int64
}

func main() {
	l1Nodes := flag.Int("l1", 4, "Number of L1 nodes")
	l2Nodes := flag.Int("l2", 2, "Number of L2 nodes")
	iterations := flag.Int("n", 100, "Number of iterations")
	l2Port := flag.String("port", "7000", "L2 port")
	packageID := flag.String("pkg", "PKG-001", "Package ID to use")
	flag.Parse()

	recordsDir := "./records"
	os.MkdirAll(recordsDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(recordsDir, fmt.Sprintf(
		"latency_%s_n%d_l1-%d_l2-%d.csv",
		timestamp, *iterations, *l1Nodes, *l2Nodes,
	))

	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Iteration", "Step", "Latency_ms", "BlockHeight"})

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", *l2Port)
	client := NewHTTPClient(baseURL)

	// Set headers for all requests
	client.SetHeaders(map[string]string{
		"X-Client-Group": "group-a",
	})

	fmt.Println("========================================")
	fmt.Println("   LATENCY BENCHMARK")
	fmt.Println("========================================")
	fmt.Printf("L1 Nodes:   %d\n", *l1Nodes)
	fmt.Printf("L2 Nodes:   %d\n", *l2Nodes)
	fmt.Printf("Iterations: %d\n", *iterations)
	fmt.Printf("L2 URL:     %s\n", baseURL)
	fmt.Printf("Package ID: %s\n", *packageID)
	fmt.Printf("Headers:    X-Client-Group=group-b\n")
	fmt.Printf("Output:     %s\n", filename)
	fmt.Println("========================================")
	fmt.Println("")

	successCount := 0
	failCount := 0

	for i := 0; i < *iterations; i++ {
		fmt.Printf("\n[%d/%d] Starting workflow...", i+1, *iterations)

		results, errMsg := runWorkflow(client, *packageID)

		if errMsg == "" {
			successCount++
			for _, r := range results {
				writer.Write([]string{
					strconv.Itoa(i + 1),
					r.Step,
					strconv.FormatInt(r.Latency.Milliseconds(), 10),
					strconv.FormatInt(r.BlockHeight, 10),
				})
			}
		} else {
			failCount++
			fmt.Printf("\n  ✗ Failed: %s\n", errMsg)
		}

		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("\n\n========================================\n")
	fmt.Printf("BENCHMARK COMPLETE\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Success: %d/%d (%.1f%%)\n", successCount, *iterations, float64(successCount)/float64(*iterations)*100)
	if failCount > 0 {
		fmt.Printf("Failed:  %d/%d (%.1f%%)\n", failCount, *iterations, float64(failCount)/float64(*iterations)*100)
	}
	fmt.Printf("Results saved to: %s\n", filename)
	fmt.Println("========================================")
}

func runWorkflow(client *HTTPClient, packageID string) ([]Result, string) {
	var results []Result
	totalStart := time.Now()

	// 1. Start Session
	start := time.Now()
	resp, err := client.POST("/session/start", map[string]interface{}{
		"operator_id": "OPR-001",
	})
	if err != nil {
		return results, fmt.Sprintf("Start Session: %v", err)
	}
	var sessResp SessionResponse
	if err := UnmarshalBody(resp, &sessResp); err != nil {
		return results, fmt.Sprintf("Start Session (unmarshal): %v", err)
	}
	sessionID := sessResp.SessionID
	fmt.Printf("\n  [1] Start Session -> SessionID: %s", sessionID)
	results = append(results, Result{"Start Session", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 2. Scan Package
	start = time.Now()
	endpoint := fmt.Sprintf("/session/%s/scan", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
	})
	if err != nil {
		return results, fmt.Sprintf("Scan Package: %v", err)
	}
	// Read response body for logging
	var scanResp map[string]interface{}
	if err := UnmarshalBody(resp, &scanResp); err != nil {
		return results, fmt.Sprintf("Scan Package (unmarshal): %v", err)
	}
	fmt.Printf("\n  [2] Scan Package -> Status: %v", scanResp["status"])
	results = append(results, Result{"Scan Package", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 3. Validate Package
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/validate", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
		"signature":  "sig_test_001",
	})
	if err != nil {
		return results, fmt.Sprintf("Validate Package: %v", err)
	}
	var validateResp map[string]interface{}
	if err := UnmarshalBody(resp, &validateResp); err != nil {
		return results, fmt.Sprintf("Validate Package (unmarshal): %v", err)
	}
	fmt.Printf("\n  [3] Validate Package -> Status: %v", validateResp["status"])
	results = append(results, Result{"Validate Package", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 4. Quality Check
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/qc", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"passed": true,
		"issues": []string{},
	})
	if err != nil {
		return results, fmt.Sprintf("Quality Check: %v", err)
	}
	var qcResp map[string]interface{}
	if err := UnmarshalBody(resp, &qcResp); err != nil {
		return results, fmt.Sprintf("Quality Check (unmarshal): %v", err)
	}
	fmt.Printf("\n  [4] Quality Check -> Status: %v", qcResp["status"])
	results = append(results, Result{"Quality Check", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 5. Label Package
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/label", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"courier_id": "CUR-001",
	})
	if err != nil {
		return results, fmt.Sprintf("Label Package: %v", err)
	}
	var labelResp map[string]interface{}
	if err := UnmarshalBody(resp, &labelResp); err != nil {
		return results, fmt.Sprintf("Label Package (unmarshal): %v", err)
	}
	fmt.Printf("\n  [5] Label Package -> Status: %v", labelResp["status"])
	results = append(results, Result{"Label Package", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 6. Commit Session
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/commit", sessionID)
	resp, err = client.POST(endpoint, nil)
	if err != nil {
		return results, fmt.Sprintf("Commit Session: %v", err)
	}
	var commitResp CommitResponse
	if err := UnmarshalBody(resp, &commitResp); err != nil {
		return results, fmt.Sprintf("Commit Session (unmarshal): %v", err)
	}
	fmt.Printf("\n  [6] Commit Session -> TxHash: %s, BlockHeight: %d", commitResp.TxHash, commitResp.BlockHeight)
	results = append(results, Result{"Commit Session", time.Since(start), commitResp.BlockHeight})

	// Total
	results = append(results, Result{"Complete Workflow", time.Since(totalStart), 0})

	fmt.Printf("\n  ✓ Workflow completed in %dms\n", time.Since(totalStart).Milliseconds())

	return results, ""
}
