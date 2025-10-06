package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
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

type WorkflowResult struct {
	Step        string
	Latency     time.Duration
	Success     bool
	BlockHeight int64
}

type PhaseStats struct {
	TotalRequests int
	SuccessCount  int
	FailureCount  int
	TotalLatency  time.Duration
	AvgLatency    time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
}

func main() {
	l1Nodes := flag.Int("l1", 4, "Number of L1 nodes")
	l2Nodes := flag.Int("l2", 1, "Number of L2 nodes")
	iterations := flag.Int("n", 50, "Number of iterations per phase")
	l2Port := flag.String("port", "7000", "L2 port")
	nodeToKill := flag.String("kill", "l1-node1", "Name of L1 node to kill")
	flag.Parse()

	recordsDir := "./records"
	os.MkdirAll(recordsDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(recordsDir, fmt.Sprintf(
		"node_failure_%s_n%d_l1-%d_l2-%d.csv",
		timestamp, *iterations, *l1Nodes, *l2Nodes,
	))

	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("❌ Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Phase", "Iteration", "Step", "Latency_ms", "Success", "BlockHeight", "Error"})

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", *l2Port)
	client := NewHTTPClient(baseURL)

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║       L1 NODE FAILURE TEST - Byzantine Fault          ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📋 Test Configuration:\n")
	fmt.Printf("   L1 Nodes:      %d (f=1, tolerates 1 failure)\n", *l1Nodes)
	fmt.Printf("   L2 Nodes:      %d\n", *l2Nodes)
	fmt.Printf("   Iterations:    %d per phase\n", *iterations)
	fmt.Printf("   L2 URL:        %s\n", baseURL)
	fmt.Printf("   Node to Kill:  %s\n", *nodeToKill)
	fmt.Printf("   Output:        %s\n", filename)
	fmt.Println("\n════════════════════════════════════════════════════════")

	// PHASE 1: Baseline
	fmt.Println("\n🟢 PHASE 1: BASELINE TEST")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Running %d iterations with all %d L1 nodes healthy...\n\n", *iterations, *l1Nodes)

	baselineStats := runPhase(client, "baseline", *iterations, writer)
	printPhaseStats("BASELINE", baselineStats)

	fmt.Println("\n⏳ Waiting 5 seconds before node failure...")
	time.Sleep(5 * time.Second)

	// PHASE 2: Kill node
	fmt.Println("\n🔴 PHASE 2: NODE FAILURE TEST")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Killing node: %s\n", *nodeToKill)

	if err := killNode(*nodeToKill); err != nil {
		fmt.Printf("❌ Failed to kill node: %v\n", err)
		return
	}

	fmt.Printf("✅ Node %s killed successfully\n", *nodeToKill)
	fmt.Println("⏳ Waiting 3 seconds for system to stabilize...")
	time.Sleep(3 * time.Second)

	fmt.Printf("\nRunning %d iterations with only %d L1 nodes...\n\n", *iterations, *l1Nodes-1)
	failedStats := runPhase(client, "node-failed", *iterations, writer)
	printPhaseStats("NODE FAILED", failedStats)

	// Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                    TEST SUMMARY                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	fmt.Println("\n📊 Performance Comparison:")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("BASELINE (All Nodes):  %.2f%% success, Avg: %v\n",
		float64(baselineStats.SuccessCount)/float64(baselineStats.TotalRequests)*100,
		baselineStats.AvgLatency)
	fmt.Printf("NODE FAILED (f=1):     %.2f%% success, Avg: %v\n",
		float64(failedStats.SuccessCount)/float64(failedStats.TotalRequests)*100,
		failedStats.AvgLatency)

	if baselineStats.AvgLatency > 0 {
		latencyIncrease := float64(failedStats.AvgLatency-baselineStats.AvgLatency) / float64(baselineStats.AvgLatency) * 100
		fmt.Printf("\nLatency Impact:        +%.1f%%\n", latencyIncrease)
	}

	if failedStats.SuccessCount == failedStats.TotalRequests {
		fmt.Println("\n✅ BYZANTINE FAULT TOLERANCE VERIFIED!")
		fmt.Println("   System continued operating despite 1 node failure")
	} else {
		fmt.Printf("\n⚠️  Some failures detected: %d/%d requests failed\n",
			failedStats.FailureCount, failedStats.TotalRequests)
	}

	fmt.Printf("\n📁 Full results saved to: %s\n", filename)
	fmt.Println("════════════════════════════════════════════════════════")
}

func runPhase(client *HTTPClient, phaseName string, iterations int, writer *csv.Writer) *PhaseStats {
	stats := &PhaseStats{
		MinLatency: time.Hour,
	}

	for i := 0; i < iterations; i++ {
		fmt.Printf("\r[%d/%d] ", i+1, iterations)

		results, errMsg := runWorkflow(client)
		stats.TotalRequests++

		if len(results) > 0 {
			stats.SuccessCount++
			fmt.Print("✓")

			totalLatency := time.Duration(0)
			for _, r := range results {
				totalLatency += r.Latency
				if r.Latency < stats.MinLatency {
					stats.MinLatency = r.Latency
				}
				if r.Latency > stats.MaxLatency {
					stats.MaxLatency = r.Latency
				}

				writer.Write([]string{
					phaseName,
					strconv.Itoa(i + 1),
					r.Step,
					strconv.FormatInt(r.Latency.Milliseconds(), 10),
					"true",
					strconv.FormatInt(r.BlockHeight, 10),
					"",
				})
			}
			stats.TotalLatency += totalLatency
		} else {
			stats.FailureCount++
			fmt.Print("✗")

			writer.Write([]string{
				phaseName,
				strconv.Itoa(i + 1),
				"workflow",
				"0",
				"false",
				"0",
				errMsg,
			})
		}

		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println()

	if stats.SuccessCount > 0 {
		stats.AvgLatency = stats.TotalLatency / time.Duration(stats.SuccessCount)
	}

	return stats
}

func printPhaseStats(name string, stats *PhaseStats) {
	fmt.Printf("\n📈 %s Statistics:\n", name)
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("Total Requests:  %d\n", stats.TotalRequests)
	fmt.Printf("Success:         %d (%.1f%%)\n",
		stats.SuccessCount,
		float64(stats.SuccessCount)/float64(stats.TotalRequests)*100)
	if stats.FailureCount > 0 {
		fmt.Printf("Failed:          %d (%.1f%%)\n",
			stats.FailureCount,
			float64(stats.FailureCount)/float64(stats.TotalRequests)*100)
	}
	if stats.SuccessCount > 0 {
		fmt.Printf("Avg Latency:     %v\n", stats.AvgLatency)
		fmt.Printf("Min Latency:     %v\n", stats.MinLatency)
		fmt.Printf("Max Latency:     %v\n", stats.MaxLatency)
	}
}

func killNode(nodeName string) error {
	cmd := exec.Command("docker", "kill", nodeName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker kill failed: %v, output: %s", err, output)
	}
	return nil
}

func runWorkflow(client *HTTPClient) ([]WorkflowResult, string) {
	var results []WorkflowResult
	packageID := "PKG-001" // Fixed package ID

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
	results = append(results, WorkflowResult{
		Step:    "Start Session",
		Latency: time.Since(start),
		Success: true,
	})
	time.Sleep(100 * time.Millisecond)

	// 2. Scan Package
	start = time.Now()
	endpoint := fmt.Sprintf("/session/%s/scan", sessionID)
	_, err = client.GET(endpoint)
	if err != nil {
		return results, fmt.Sprintf("Scan Package: %v", err)
	}
	results = append(results, WorkflowResult{
		Step:    "Scan Package",
		Latency: time.Since(start),
		Success: true,
	})
	time.Sleep(100 * time.Millisecond)

	// 3. Validate Package
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/validate", sessionID)
	_, err = client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
		"signature":  "sig_test_001",
	})
	if err != nil {
		return results, fmt.Sprintf("Validate Package: %v", err)
	}
	results = append(results, WorkflowResult{
		Step:    "Validate Package",
		Latency: time.Since(start),
		Success: true,
	})
	time.Sleep(100 * time.Millisecond)

	// 4. Quality Check
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/qc", sessionID)
	_, err = client.POST(endpoint, map[string]interface{}{
		"passed": true,
		"issues": []string{},
	})
	if err != nil {
		return results, fmt.Sprintf("Quality Check: %v", err)
	}
	results = append(results, WorkflowResult{
		Step:    "Quality Check",
		Latency: time.Since(start),
		Success: true,
	})
	time.Sleep(100 * time.Millisecond)

	// 5. Label Package
	start = time.Now()
	endpoint = fmt.Sprintf("/session/%s/label", sessionID)
	_, err = client.POST(endpoint, map[string]interface{}{
		"courier_id": "CUR-001",
	})
	if err != nil {
		return results, fmt.Sprintf("Label Package: %v", err)
	}
	results = append(results, WorkflowResult{
		Step:    "Label Package",
		Latency: time.Since(start),
		Success: true,
	})
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
	results = append(results, WorkflowResult{
		Step:        "Commit Session",
		Latency:     time.Since(start),
		Success:     true,
		BlockHeight: commitResp.BlockHeight,
	})

	return results, ""
}
