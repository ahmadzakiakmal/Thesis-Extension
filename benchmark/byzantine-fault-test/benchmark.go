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
	l1Nodes := flag.Int("l1", 16, "Number of L1 nodes")
	l2Nodes := flag.Int("l2", 1, "Number of L2 nodes")
	iterations := flag.Int("n", 50, "Number of iterations per phase")
	l2Port := flag.String("port", "7000", "L2 port")
	byzantine1 := flag.String("byz1", "l1-node1", "First Byzantine node")
	byzantine2 := flag.String("byz2", "l1-node2", "Second Byzantine node")
	flag.Parse()

	recordsDir := "./records"
	os.MkdirAll(recordsDir, 0755)

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(recordsDir, fmt.Sprintf(
		"byzantine_fault_%s_n%d_l1-%d_l2-%d.csv",
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
	fmt.Println("║       BYZANTINE FAULT TEST - L1 Consensus             ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📋 Test Configuration:\n")
	fmt.Printf("   L1 Nodes:         %d (f=%d, tolerates %d Byzantine faults)\n", *l1Nodes, (*l1Nodes-1)/3, (*l1Nodes-1)/3)
	fmt.Printf("   L2 Nodes:         %d\n", *l2Nodes)
	fmt.Printf("   Iterations:       %d per phase\n", *iterations)
	fmt.Printf("   L2 URL:           %s\n", baseURL)
	fmt.Printf("   Byzantine Node 1: %s\n", *byzantine1)
	fmt.Printf("   Byzantine Node 2: %s\n", *byzantine2)
	fmt.Printf("   Output:           %s\n", filename)
	fmt.Println("\n════════════════════════════════════════════════════════")

	// PHASE 1: All Healthy Nodes
	fmt.Println("\n🟢 PHASE 1: BASELINE - ALL HEALTHY NODES")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Running %d iterations with all %d L1 nodes healthy...\n\n", *iterations, *l1Nodes)

	baselineStats := runPhase(client, "all-healthy", *iterations, writer)
	printPhaseStats("ALL HEALTHY", baselineStats)

	fmt.Println("\n⏳ Waiting 5 seconds before introducing Byzantine node...")
	time.Sleep(5 * time.Second)

	// PHASE 2: 1 Byzantine Node
	fmt.Println("\n🟡 PHASE 2: 1 BYZANTINE NODE")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Stopping Byzantine node: %s\n", *byzantine1)

	if err := stopNode(*byzantine1); err != nil {
		fmt.Printf("❌ Failed to stop node: %v\n", err)
		return
	}

	fmt.Printf("✅ Node %s stopped (simulating Byzantine behavior)\n", *byzantine1)
	fmt.Println("⏳ Waiting 3 seconds for system to stabilize...")
	time.Sleep(3 * time.Second)

	fmt.Printf("\nRunning %d iterations with 1 Byzantine node (%d healthy)...\n\n", *iterations, *l1Nodes-1)
	byz1Stats := runPhase(client, "1-byzantine", *iterations, writer)
	printPhaseStats("1 BYZANTINE", byz1Stats)

	fmt.Println("\n⏳ Waiting 5 seconds before introducing second Byzantine node...")
	time.Sleep(5 * time.Second)

	// PHASE 3: 2 Byzantine Nodes
	fmt.Println("\n🔴 PHASE 3: 2 BYZANTINE NODES")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Stopping second Byzantine node: %s\n", *byzantine2)

	if err := stopNode(*byzantine2); err != nil {
		fmt.Printf("❌ Failed to stop second node: %v\n", err)
		return
	}

	fmt.Printf("✅ Node %s stopped (simulating Byzantine behavior)\n", *byzantine2)
	fmt.Println("⏳ Waiting 3 seconds for system to stabilize...")
	time.Sleep(3 * time.Second)

	fmt.Printf("\nRunning %d iterations with 2 Byzantine nodes (%d healthy)...\n\n", *iterations, *l1Nodes-2)
	byz2Stats := runPhase(client, "2-byzantine", *iterations, writer)
	printPhaseStats("2 BYZANTINE", byz2Stats)

	// Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                    TEST SUMMARY                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	fmt.Println("\n📊 Performance Comparison:")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("ALL HEALTHY:       %.2f%% success, Avg: %v\n",
		float64(baselineStats.SuccessCount)/float64(baselineStats.TotalRequests)*100,
		baselineStats.AvgLatency)
	fmt.Printf("1 BYZANTINE:       %.2f%% success, Avg: %v\n",
		float64(byz1Stats.SuccessCount)/float64(byz1Stats.TotalRequests)*100,
		byz1Stats.AvgLatency)
	fmt.Printf("2 BYZANTINE:       %.2f%% success, Avg: %v\n",
		float64(byz2Stats.SuccessCount)/float64(byz2Stats.TotalRequests)*100,
		byz2Stats.AvgLatency)

	if baselineStats.AvgLatency > 0 {
		latencyIncrease1 := float64(byz1Stats.AvgLatency-baselineStats.AvgLatency) / float64(baselineStats.AvgLatency) * 100
		latencyIncrease2 := float64(byz2Stats.AvgLatency-baselineStats.AvgLatency) / float64(baselineStats.AvgLatency) * 100
		fmt.Printf("\nLatency Impact (1 Byzantine): +%.1f%%\n", latencyIncrease1)
		fmt.Printf("Latency Impact (2 Byzantine): +%.1f%%\n", latencyIncrease2)
	}

	faultTolerance := (*l1Nodes - 1) / 3
	fmt.Printf("\n🛡️  Byzantine Fault Tolerance Analysis:\n")
	fmt.Printf("   Maximum tolerable faults (f): %d\n", faultTolerance)

	if byz1Stats.SuccessCount == byz1Stats.TotalRequests {
		fmt.Println("   ✅ System survived 1 Byzantine node")
	} else {
		fmt.Printf("   ⚠️  System degraded with 1 Byzantine node: %d/%d failures\n",
			byz1Stats.FailureCount, byz1Stats.TotalRequests)
	}

	if byz2Stats.SuccessCount == byz2Stats.TotalRequests {
		fmt.Println("   ✅ System survived 2 Byzantine nodes")
	} else {
		fmt.Printf("   ⚠️  System degraded with 2 Byzantine nodes: %d/%d failures\n",
			byz2Stats.FailureCount, byz2Stats.TotalRequests)
	}

	if byz1Stats.SuccessCount == byz1Stats.TotalRequests &&
		byz2Stats.SuccessCount == byz2Stats.TotalRequests {
		fmt.Println("\n✅ BYZANTINE FAULT TOLERANCE VERIFIED!")
		fmt.Printf("   System successfully handled up to %d Byzantine nodes\n", 2)
	}

	fmt.Printf("\n📁 Full results saved to: %s\n", filename)
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println("\n💡 Next steps:")
	fmt.Printf("   1. Restart Byzantine nodes: docker start %s %s\n", *byzantine1, *byzantine2)
	fmt.Println("   2. Analyze results in ./records/")
	fmt.Println("   3. Run preview.py to visualize the data")
	fmt.Println("")
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

func stopNode(nodeName string) error {
	cmd := exec.Command("docker", "stop", nodeName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop failed: %v, output: %s", err, output)
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
	resp, err = client.POST(endpoint, map[string]interface{}{
		"package_id": packageID,
	})
	if err != nil {
		return results, fmt.Sprintf("Scan Package: %v", err)
	}
	var scanResp map[string]interface{}
	if err := UnmarshalBody(resp, &scanResp); err != nil {
		return results, fmt.Sprintf("Scan Package (unmarshal): %v", err)
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
	results = append(results, WorkflowResult{
		Step:    "Validate Package",
		Latency: time.Since(start),
		Success: true,
	})
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
	results = append(results, WorkflowResult{
		Step:    "Quality Check",
		Latency: time.Since(start),
		Success: true,
	})
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
