package l1client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-2/repository/models"
)

// L1Client handles communication with L1 BFT network
type L1Client struct {
	endpoint   string
	httpClient *http.Client
	shardID    string
	nodeID     string
}

// CommitRequest represents the request to commit a session to L1
type CommitRequest struct {
	ShardID     string                 `json:"shard_id"`
	ClientGroup string                 `json:"client_group"`
	SessionID   string                 `json:"session_id"`
	OperatorID  string                 `json:"operator_id"`
	SessionData map[string]interface{} `json:"session_data"`
	L2NodeID    string                 `json:"l2_node_id"`
	Timestamp   time.Time              `json:"timestamp"`
}

// CommitResponse represents the response from L1
type CommitResponse struct {
	Data struct {
		Message   string `json:"message"`
		TxHash    string `json:"tx_hash"`
		SessionID string `json:"session_id"`
		ShardID   string `json:"shard_id"`
	} `json:"data"`
	Meta struct {
		TxID        string    `json:"tx_id"`
		Status      string    `json:"status"`
		BlockHeight int64     `json:"block_height"`
		ConfirmTime time.Time `json:"confirm_time"`
		ShardInfo   struct {
			ShardID     string `json:"shard_id"`
			ClientGroup string `json:"client_group"`
			L2NodeID    string `json:"l2_node_id"`
		} `json:"shard_info"`
	} `json:"meta"`
	NodeID string `json:"node_id"`
}

// NewL1Client creates a new L1 client
func NewL1Client(endpoint, shardID, nodeID string) *L1Client {
	return &L1Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		shardID: shardID,
		nodeID:  nodeID,
	}
}

// CommitSession commits a completed session to L1
func (c *L1Client) CommitSession(session *models.Session, clientGroup string) (*CommitResponse, error) {
	// Build session data
	sessionData := c.buildSessionData(session)

	// Create commit request
	commitReq := CommitRequest{
		ShardID:     c.shardID,
		ClientGroup: clientGroup,
		SessionID:   session.ID,
		OperatorID:  session.OperatorID,
		SessionData: sessionData,
		L2NodeID:    c.nodeID,
		Timestamp:   time.Now(),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(commitReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal commit request: %w", err)
	}

	// Make HTTP request to L1
	url := fmt.Sprintf("%s/l1/commit", c.endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to L1: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read L1 response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("L1 returned error status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var commitResp CommitResponse
	if err := json.Unmarshal(body, &commitResp); err != nil {
		return nil, fmt.Errorf("failed to parse L1 response: %w", err)
	}

	return &commitResp, nil
}

// buildSessionData builds the session data payload for L1
func (c *L1Client) buildSessionData(session *models.Session) map[string]interface{} {
	data := map[string]interface{}{
		"session_id":  session.ID,
		"operator_id": session.OperatorID,
		"status":      session.Status,
		"created_at":  session.CreatedAt,
		"updated_at":  session.UpdatedAt,
	}

	// Add package info if exists
	if session.Package != nil {
		packageData := map[string]interface{}{
			"package_id": session.Package.ID,
			"signature":  session.Package.Signature,
			"supplier":   nil,
			"items":      []map[string]interface{}{},
		}

		// Add supplier info
		if session.Package.Supplier != nil {
			packageData["supplier"] = map[string]interface{}{
				"supplier_id": session.Package.Supplier.ID,
				"name":        session.Package.Supplier.Name,
				"country":     session.Package.Supplier.Country,
			}
		}

		// Add items
		if len(session.Package.Items) > 0 {
			items := []map[string]interface{}{}
			for _, item := range session.Package.Items {
				items = append(items, map[string]interface{}{
					"item_id":     item.ID,
					"description": item.Description,
					"quantity":    item.Quantity,
				})
			}
			packageData["items"] = items
		}

		data["package"] = packageData
	}

	// Add QC record if exists
	if session.QCRecord != nil {
		data["qc_record"] = map[string]interface{}{
			"qc_id":      session.QCRecord.ID,
			"passed":     session.QCRecord.Passed,
			"issues":     session.QCRecord.Issues,
			"created_at": session.QCRecord.CreatedAt,
		}
	}

	// Add label if exists
	if session.Label != nil {
		labelData := map[string]interface{}{
			"label_id":    session.Label.ID,
			"tracking_no": session.Label.TrackingNo,
			"created_at":  session.Label.CreatedAt,
		}

		if session.Label.Courier != nil {
			labelData["courier"] = map[string]interface{}{
				"courier_id": session.Label.Courier.ID,
				"name":       session.Label.Courier.Name,
			}
		}

		data["label"] = labelData
	}

	return data
}

// HealthCheck checks if L1 is reachable
func (c *L1Client) HealthCheck() error {
	url := fmt.Sprintf("%s/l1/status", c.endpoint)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("L1 is unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("L1 health check failed with status: %d", resp.StatusCode)
	}

	return nil
}
