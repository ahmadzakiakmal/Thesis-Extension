package srvreg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// InfoHandler returns shard information
func (sr *ServiceRegistry) InfoHandler(req *Request) (*Response, error) {
	info := map[string]interface{}{
		"shard_id":     sr.shardID,
		"client_group": sr.clientGroup,
		"type":         "L2 Shard Node",
		"status":       "active",
	}

	body, _ := json.Marshal(info)

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(body),
	}, nil
}

// CreateSessionHandler creates a new session
func (sr *ServiceRegistry) CreateSessionHandler(req *Request) (*Response, error) {
	var body struct {
		OperatorID string `json:"operator_id"`
	}

	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid request body: %s"}`, err.Error()),
		}, nil
	}

	if body.OperatorID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"operator_id is required"}`,
		}, nil
	}

	session, dbErr := sr.repository.CreateSession(body.OperatorID)
	if dbErr != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Failed to create session: %s"}`, dbErr.Message),
		}, nil
	}

	return &Response{
		StatusCode: http.StatusCreated,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
			"message":"Session created successfully",
			"session_id":"%s",
			"operator_id":"%s",
			"status":"%s",
			"shard_id":"%s"
		}`, session.ID, session.OperatorID, session.Status, sr.shardID),
	}, nil
}

// ScanPackageHandler scans a package
func (sr *ServiceRegistry) ScanPackageHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, nil
	}
	sessionID := pathParts[2]

	var body struct {
		PackageID string `json:"package_id"`
	}

	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid request body: %s"}`, err.Error()),
		}, nil
	}

	if body.PackageID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"package_id is required"}`,
		}, nil
	}

	pkg, dbErr := sr.repository.ScanPackage(sessionID, body.PackageID)
	if dbErr != nil {
		statusCode := http.StatusInternalServerError
		if dbErr.Code == "NOT_FOUND" {
			statusCode = http.StatusNotFound
		}
		return &Response{
			StatusCode: statusCode,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
		}, nil
	}

	// Format items
	items := []map[string]interface{}{}
	for _, item := range pkg.Items {
		items = append(items, map[string]interface{}{
			"item_id":     item.ID,
			"description": item.Description,
			"quantity":    item.Quantity,
		})
	}

	supplierName := "Unknown"
	if pkg.Supplier != nil {
		supplierName = pkg.Supplier.Name
	}

	response := map[string]interface{}{
		"message":            "Package scanned successfully",
		"package_id":         pkg.ID,
		"supplier":           supplierName,
		"expected_contents":  items,
		"supplier_signature": pkg.Signature,
		"status":             pkg.Status,
		"next_step":          "validate",
	}

	body_bytes, _ := json.Marshal(response)

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(body_bytes),
	}, nil
}

// ValidatePackageHandler validates package signature
func (sr *ServiceRegistry) ValidatePackageHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, nil
	}
	sessionID := pathParts[2]

	var body struct {
		Signature string `json:"signature"`
		PackageID string `json:"package_id"`
	}

	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid request body: %s"}`, err.Error()),
		}, nil
	}

	if body.Signature == "" || body.PackageID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"signature and package_id are required"}`,
		}, nil
	}

	pkg, dbErr := sr.repository.ValidatePackage(body.Signature, body.PackageID, sessionID)
	if dbErr != nil {
		statusCode := http.StatusInternalServerError
		if dbErr.Code == "NOT_FOUND" {
			statusCode = http.StatusNotFound
		}
		return &Response{
			StatusCode: statusCode,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
		}, nil
	}

	supplierName := "Unknown"
	if pkg.Supplier != nil {
		supplierName = pkg.Supplier.Name
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
			"message":"Package validated successfully",
			"package_id":"%s",
			"supplier":"%s",
			"is_trusted":%t,
			"status":"%s",
			"next_step":"qc"
		}`, pkg.ID, supplierName, pkg.IsTrusted, pkg.Status),
	}, nil
}

// QualityCheckHandler performs quality check
func (sr *ServiceRegistry) QualityCheckHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, nil
	}
	sessionID := pathParts[2]

	var body struct {
		Passed bool     `json:"passed"`
		Issues []string `json:"issues"`
	}

	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid request body: %s"}`, err.Error()),
		}, nil
	}

	pkg, qcRecord, dbErr := sr.repository.QualityCheck(sessionID, body.Passed, body.Issues)
	if dbErr != nil {
		statusCode := http.StatusInternalServerError
		if dbErr.Code == "NOT_FOUND" {
			statusCode = http.StatusNotFound
		}
		return &Response{
			StatusCode: statusCode,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
		}, nil
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
			"message":"Quality check completed",
			"qc_id":"%s",
			"passed":%t,
			"package_id":"%s",
			"status":"%s",
			"next_step":"label"
		}`, qcRecord.ID, qcRecord.Passed, pkg.ID, pkg.Status),
	}, nil
}

// LabelPackageHandler creates shipping label
func (sr *ServiceRegistry) LabelPackageHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, nil
	}
	sessionID := pathParts[2]

	var body struct {
		CourierID string `json:"courier_id"`
	}

	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid request body: %s"}`, err.Error()),
		}, nil
	}

	if body.CourierID == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"courier_id is required"}`,
		}, nil
	}

	label, dbErr := sr.repository.LabelPackage(sessionID, body.CourierID)
	if dbErr != nil {
		statusCode := http.StatusInternalServerError
		if dbErr.Code == "NOT_FOUND" {
			statusCode = http.StatusNotFound
		}
		return &Response{
			StatusCode: statusCode,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
		}, nil
	}

	courierName := "Unknown"
	if label.Courier != nil {
		courierName = label.Courier.Name
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
			"message":"Shipping label created",
			"label_id":"%s",
			"tracking_no":"%s",
			"courier":"%s",
			"session_id":"%s",
			"next_step":"commit"
		}`, label.ID, label.TrackingNo, courierName, sessionID),
	}, nil
}

// CommitSessionHandler commits session to L1
func (sr *ServiceRegistry) CommitSessionHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, nil
	}
	sessionID := pathParts[2]

	// Get session with all related data
	session, dbErr := sr.repository.GetSession(sessionID)
	if dbErr != nil {
		statusCode := http.StatusInternalServerError
		if dbErr.Code == "NOT_FOUND" {
			statusCode = http.StatusNotFound
		}
		return &Response{
			StatusCode: statusCode,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"%s"}`, dbErr.Message),
		}, nil
	}

	// Check if session is already committed
	if session.IsCommitted {
		return &Response{
			StatusCode: http.StatusConflict,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Session already committed","tx_hash":"%s"}`, *session.L1TxHash),
		}, nil
	}

	// Check if session is completed
	if session.Status != "completed" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Session must be completed before committing","current_status":"%s"}`, session.Status),
		}, nil
	}

	// Commit to L1
	l1Response, err := sr.l1Client.CommitSession(session, sr.clientGroup)
	if err != nil {
		return &Response{
			StatusCode: http.StatusBadGateway,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Failed to commit to L1: %s"}`, err.Error()),
		}, nil
	}

	// Update session with L1 commitment info
	dbErr = sr.repository.MarkSessionCommitted(sessionID, l1Response.Data.TxHash, l1Response.Meta.BlockHeight)
	if dbErr != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Failed to update session: %s"}`, dbErr.Message),
		}, nil
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
			"message":"Session committed to L1 successfully",
			"session_id":"%s",
			"tx_hash":"%s",
			"block_height":%d,
			"shard_id":"%s",
			"status":"committed"
		}`, sessionID, l1Response.Data.TxHash, l1Response.Meta.BlockHeight, sr.shardID),
	}, nil
}
