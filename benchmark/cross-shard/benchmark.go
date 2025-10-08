package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
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
		"cross-shard_latency_%s_n%d_l1-%d_l2-%d.csv",
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

	fmt.Println("========================================")
	fmt.Println("   CROSS-SHARD LATENCY BENCHMARK")
	fmt.Println("========================================")
	fmt.Printf("L1 Nodes:   %d\n", *l1Nodes)
	fmt.Printf("L2 Nodes:   %d\n", *l2Nodes)
	fmt.Printf("Iterations: %d\n", *iterations)
	fmt.Printf("L2 URL:     %s\n", baseURL)
	fmt.Printf("Package ID: %s\n", *packageID)
	fmt.Printf("Header:     X-Client-Group: group-b (CROSS-SHARD)\n")
	fmt.Printf("Output:     %s\n", filename)
	fmt.Println("========================================")
	fmt.Println("")

	successCount := 0
	failCount := 0

	for i := 0; i < *iterations; i++ {
		fmt.Printf("\n[%d/%d] Starting cross-shard workflow...", i+1, *iterations)

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
	fmt.Printf("CROSS-SHARD BENCHMARK COMPLETE\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Success: %d/%d (%.1f%%)\n", successCount, *iterations, float64(successCount)/float64(*iterations)*100)
	if failCount > 0 {
		fmt.Printf("Failed:  %d/%d (%.1f%%)\n", failCount, *iterations, float64(failCount)/float64(*iterations)*100)
	}
	fmt.Printf("Results saved to: %s\n", filename)
	fmt.Println("========================================")
}

// Helper function to pretty print JSON response
func logResponseBody(stepName string, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("\n      → Error reading response: %v", err)
		return
	}

	// Pretty print JSON
	var prettyJSON map[string]interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		if msg, ok := prettyJSON["message"].(string); ok {
			fmt.Printf("\n      → Message: %s", msg)
		}
		if status, ok := prettyJSON["status"].(string); ok {
			fmt.Printf("\n      → Status: %s", status)
		}
		if shardID, ok := prettyJSON["shard_id"].(string); ok {
			fmt.Printf("\n      → Shard ID: %s", shardID)
		}
	}

	fmt.Printf("\n      → Response Body: %s", string(body))
}

func runWorkflow(client *HTTPClient, packageID string) ([]Result, string) {
	var results []Result
	totalStart := time.Now()

	// WRONG CLIENT GROUP - This will trigger cross-shard forwarding!
	// Sending to shard A (localhost:7000) but with group-b header
	headers := map[string]string{
		"X-Client-Group": "group-b",
	}

	// 1. Start Session
	start := time.Now()
	resp, err := client.POST("/session/start", map[string]interface{}{
		"operator_id": "OPR-001",
	}, headers)
	if err != nil {
		return results, fmt.Sprintf("Start Session: %v", err)
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return results, fmt.Sprintf("Start Session (read): %v", err)
	}

	var sessResp SessionResponse
	if err := json.Unmarshal(bodyBytes, &sessResp); err != nil {
		return results, fmt.Sprintf("Start Session (unmarshal): %v", err)
	}

	sessionID := sessResp.SessionID
	fmt.Printf("\n  [1] Start Session")
	fmt.Printf("\n      → SessionID: %s", sessionID)
	fmt.Printf("\n      → [FORWARDED to Shard B]")
	fmt.Printf("\n      → Response Body: %s", string(bodyBytes))
	results = append(results, Result{"Start Session", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 2. Scan Package
	start = time.Now()
	endpoint := fmt.Sprintf("/session/%s/scan", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
	}, headers)
	if err != nil {
		return results, fmt.Sprintf("Scan Package: %v", err)
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return results, fmt.Sprintf("Scan Package (read): %v", err)
	}

	var scanResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &scanResp); err != nil {
		return results, fmt.Sprintf("Scan Package (unmarshal): %v", err)
	}

	fmt.Printf("\n  [2] Scan Package")
	fmt.Printf("\n      → Status: %v", scanResp["status"])
	fmt.Printf("\n      → Message: %v", scanResp["message"])
	fmt.Printf("\n      → [FORWARDED to Shard B]")
	fmt.Printf("\n      → Response Body: %s", string(bodyBytes))
	results = append(results, Result{"Scan Package", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 3. Validate Package
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/validate", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
		"signature":  "sig_test_001",
	}, headers)
	if err != nil {
		return results, fmt.Sprintf("Validate Package: %v", err)
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return results, fmt.Sprintf("Validate Package (read): %v", err)
	}

	var validateResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &validateResp); err != nil {
		return results, fmt.Sprintf("Validate Package (unmarshal): %v", err)
	}

	fmt.Printf("\n  [3] Validate Package")
	fmt.Printf("\n      → Status: %v", validateResp["status"])
	fmt.Printf("\n      → Message: %v", validateResp["message"])
	fmt.Printf("\n      → [FORWARDED to Shard B]")
	fmt.Printf("\n      → Response Body: %s", string(bodyBytes))
	results = append(results, Result{"Validate Package", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 4. Quality Check
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/qc", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"passed": true,
		"issues": []string{},
	}, headers)
	if err != nil {
		return results, fmt.Sprintf("Quality Check: %v", err)
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return results, fmt.Sprintf("Quality Check (read): %v", err)
	}

	var qcResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &qcResp); err != nil {
		return results, fmt.Sprintf("Quality Check (unmarshal): %v", err)
	}

	fmt.Printf("\n  [4] Quality Check")
	fmt.Printf("\n      → Status: %v", qcResp["status"])
	fmt.Printf("\n      → Result: %v", qcResp["result"])
	fmt.Printf("\n      → [FORWARDED to Shard B]")
	fmt.Printf("\n      → Response Body: %s", string(bodyBytes))
	results = append(results, Result{"Quality Check", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 5. Label Package
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/label", sessionID)
	resp, err = client.POST(endpoint, map[string]interface{}{
		"courier_id": "CUR-001",
	}, headers)
	if err != nil {
		return results, fmt.Sprintf("Label Package: %v", err)
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return results, fmt.Sprintf("Label Package (read): %v", err)
	}

	var labelResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &labelResp); err != nil {
		return results, fmt.Sprintf("Label Package (unmarshal): %v", err)
	}

	fmt.Printf("\n  [5] Label Package")
	fmt.Printf("\n      → Status: %v", labelResp["status"])
	fmt.Printf("\n      → Courier: %v", labelResp["courier_id"])
	fmt.Printf("\n      → [FORWARDED to Shard B]")
	fmt.Printf("\n      → Response Body: %s", string(bodyBytes))
	results = append(results, Result{"Label Package", time.Since(start), 0})
	time.Sleep(100 * time.Millisecond)

	// 6. Commit Session
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/commit", sessionID)
	resp, err = client.POST(endpoint, nil, headers)
	if err != nil {
		return results, fmt.Sprintf("Commit Session: %v", err)
	}

	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return results, fmt.Sprintf("Commit Session (read): %v", err)
	}

	var commitResp CommitResponse
	if err := json.Unmarshal(bodyBytes, &commitResp); err != nil {
		return results, fmt.Sprintf("Commit Session (unmarshal): %v", err)
	}

	fmt.Printf("\n  [6] Commit Session")
	fmt.Printf("\n      → TxHash: %s", commitResp.TxHash)
	fmt.Printf("\n      → BlockHeight: %d", commitResp.BlockHeight)
	fmt.Printf("\n      → [L1 Consensus]")
	fmt.Printf("\n      → Response Body: %s", string(bodyBytes))
	results = append(results, Result{"Commit Session", time.Since(start), commitResp.BlockHeight})

	// Total
	results = append(results, Result{"Complete Workflow", time.Since(totalStart), 0})

	fmt.Printf("\n  ✓ Cross-shard workflow completed in %dms", time.Since(totalStart).Milliseconds())
	fmt.Printf("\n    (All requests forwarded from Shard A → Shard B)\n")

	return results, ""
}
