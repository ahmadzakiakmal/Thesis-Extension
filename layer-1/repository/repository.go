package repository

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-1/repository/models"
	cmtrpc "github.com/cometbft/cometbft/rpc/client/local"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgreSQL error codes
const (
	PgErrForeignKeyViolation = "23503"
	PgErrUniqueViolation     = "23505"
)

// ConsensusPayload represents data for L1 BFT consensus
type ConsensusPayload interface{}

// ConsensusResult contains the result of L1 consensus
type ConsensusResult struct {
	TxHash      string
	BlockHeight int64
	Code        uint32
	Error       error
}

// RepositoryError represents repository layer errors
type RepositoryError struct {
	Code    string
	Message string
	Detail  string
}

// ShardedCommitRequest represents commit from L2 shard
type ShardedCommitRequest struct {
	ShardID     string                 `json:"shard_id"`
	ClientGroup string                 `json:"client_group"`
	SessionID   string                 `json:"session_id"`
	OperatorID  string                 `json:"operator_id"`
	SessionData map[string]interface{} `json:"session_data"`
	L2NodeID    string                 `json:"l2_node_id"`
	Timestamp   time.Time              `json:"timestamp"`
}

type Repository struct {
	db        *gorm.DB
	rpcClient *cmtrpc.Local
}

func NewRepository() *Repository {
	return &Repository{}
}

// ConnectDB establishes database connection and performs migrations
func (r *Repository) ConnectDB(dsn string) {
	for i := range 10 {
		log.Printf("Connection attempt %d...\n", i+1)
		DB, err := gorm.Open(postgres.Open(dsn))
		if err != nil {
			log.Printf("Connection attempt %d, failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		r.db = DB
		break
	}

	if r.db != nil {
		r.Migrate()
		r.Seed()
		log.Println("Connected to DB and completed setup")
	} else {
		log.Println("Failed to connect to DB")
	}
}

// Migrate performs database schema migrations
func (r *Repository) Migrate() {
	migrator := r.db.Migrator()

	// 1. ShardInfo has no dependencies - create it first
	if !migrator.HasTable(&models.ShardInfo{}) {
		if err := migrator.CreateTable(&models.ShardInfo{}); err != nil {
			log.Printf("Error creating ShardInfo table: %v", err)
			return
		}
		log.Println("✓ ShardInfo table created")
	} else {
		log.Println("✓ ShardInfo table already exists")
	}

	// 2. Operator depends on ShardInfo
	if !migrator.HasTable(&models.Operator{}) {
		if err := migrator.CreateTable(&models.Operator{}); err != nil {
			log.Printf("Error creating Operator table: %v", err)
			return
		}
		log.Println("✓ Operator table created")
	} else {
		log.Println("✓ Operator table already exists")
	}

	// 3. Session depends on ShardInfo
	if !migrator.HasTable(&models.Session{}) {
		if err := migrator.CreateTable(&models.Session{}); err != nil {
			log.Printf("Error creating Session table: %v", err)
			return
		}
		log.Println("✓ Session table created")
	} else {
		log.Println("✓ Session table already exists")
	}

	// 4. Transaction depends on ShardInfo and Session
	if !migrator.HasTable(&models.Transaction{}) {
		if err := migrator.CreateTable(&models.Transaction{}); err != nil {
			log.Printf("Error creating Transaction table: %v", err)
			return
		}
		log.Println("✓ Transaction table created")
	} else {
		log.Println("✓ Transaction table already exists")
	}

	log.Println("Database migration completed successfully")
}

// Seed initializes database with test data
func (r *Repository) Seed() {
	// Check if data already exists
	var shardCount int64
	r.db.Model(&models.ShardInfo{}).Count(&shardCount)
	if shardCount > 0 {
		log.Println("Seed data already exists, skipping...")
		return
	}

	log.Println("Seeding database with shard data...")

	// Create shard info
	shards := []models.ShardInfo{
		{ShardID: "shard-a", ClientGroup: "group-a", L2NodeID: "l2-node-a", Status: "active"},
		{ShardID: "shard-b", ClientGroup: "group-b", L2NodeID: "l2-node-b", Status: "active"},
	}

	for _, shard := range shards {
		if err := r.db.Create(&shard).Error; err != nil {
			log.Printf("Error creating shard %s: %v", shard.ShardID, err)
		}
	}

	// Create cross-shard operators
	operators := []models.Operator{
		{ID: "OPR-001", Name: "John Smith", Role: "Warehouse Manager", AccessLevel: "Admin", ShardID: "shard-a"},
		{ID: "OPR-002", Name: "Sarah Lee", Role: "Quality Control", AccessLevel: "Standard", ShardID: "shard-a"},
		{ID: "OPR-003", Name: "Raj Patel", Role: "Logistics Coordinator", AccessLevel: "Standard", ShardID: "shard-b"},
		{ID: "OPR-004", Name: "Maria Garcia", Role: "Inventory Clerk", AccessLevel: "Basic", ShardID: "shard-b"},
	}

	for _, operator := range operators {
		if err := r.db.Create(&operator).Error; err != nil {
			log.Printf("Error creating operator %s: %v", operator.ID, err)
		}
	}

	log.Println("Database seeding completed successfully")
}

// SetupRpcClient configures the RPC client for BFT consensus
func (r *Repository) SetupRpcClient(rpcClient *cmtrpc.Local) {
	r.rpcClient = rpcClient
}

// ReceiveShardCommit handles commits from L2 shards
func (r *Repository) ReceiveShardCommit(commitReq *ShardedCommitRequest) (*models.Transaction, *RepositoryError) {
	dbTx := r.db.Begin()
	if dbTx.Error != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to start transaction",
			Detail:  dbTx.Error.Error(),
		}
	}

	// Verify shard exists
	var shard models.ShardInfo
	err := dbTx.Where("shard_id = ?", commitReq.ShardID).First(&shard).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "SHARD_NOT_FOUND",
				Message: "Unknown shard",
				Detail:  fmt.Sprintf("Shard %s not registered in L1", commitReq.ShardID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Convert session data to JSON
	sessionDataBytes, err := json.Marshal(commitReq.SessionData)
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "SERIALIZATION_ERROR",
			Message: "Failed to serialize session data",
			Detail:  err.Error(),
		}
	}

	// Create session record
	session := models.Session{
		ID:          commitReq.SessionID,
		ShardID:     commitReq.ShardID,
		ClientGroup: commitReq.ClientGroup,
		OperatorID:  commitReq.OperatorID,
		Status:      "committed",
		IsCommitted: true,
		SessionData: string(sessionDataBytes),
	}

	err = dbTx.Create(&session).Error
	if err != nil {
		dbTx.Rollback()
		pgErr, isPgError := err.(*pgconn.PgError)
		if isPgError && pgErr.Code == PgErrUniqueViolation {
			return nil, &RepositoryError{
				Code:    "SESSION_EXISTS",
				Message: "Session already exists",
				Detail:  fmt.Sprintf("Session %s already committed", commitReq.SessionID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to create session",
			Detail:  err.Error(),
		}
	}

	// Commit to database first
	err = dbTx.Commit().Error
	if err != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to commit database transaction",
			Detail:  err.Error(),
		}
	}

	// Now run L1 BFT consensus
	consensusResult, repoErr := r.RunConsensus(context.Background(), commitReq)
	if repoErr != nil {
		// Rollback session if consensus fails
		r.db.Delete(&session)
		return nil, repoErr
	}

	// Update session with transaction hash and create transaction record
	dbTx = r.db.Begin()

	session.TxHash = &consensusResult.TxHash
	err = dbTx.Save(&session).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to update session with tx hash",
			Detail:  err.Error(),
		}
	}

	// Create transaction record
	transaction := models.Transaction{
		TxHash:      consensusResult.TxHash,
		SessionID:   commitReq.SessionID,
		ShardID:     commitReq.ShardID,
		ClientGroup: commitReq.ClientGroup,
		BlockHeight: consensusResult.BlockHeight,
		Status:      "confirmed",
		Timestamp:   time.Now(),
	}

	err = dbTx.Create(&transaction).Error
	if err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to create transaction record",
			Detail:  err.Error(),
		}
	}

	err = dbTx.Commit().Error
	if err != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to commit final transaction",
			Detail:  err.Error(),
		}
	}

	return &transaction, nil
}

// RunConsensus submits data to L1 BFT consensus
func (r *Repository) RunConsensus(ctx context.Context, payload ConsensusPayload) (*ConsensusResult, *RepositoryError) {
	// Serialize the payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, &RepositoryError{
			Code:    "SERIALIZATION_ERROR",
			Message: "Failed to serialize consensus payload",
			Detail:  err.Error(),
		}
	}

	// Create consensus transaction
	consensusTx := cmttypes.Tx(payloadBytes)

	// Use a channel for async consensus
	done := make(chan struct {
		result *cmtrpctypes.ResultBroadcastTxCommit
		err    error
	}, 1)

	go func() {
		result, err := r.rpcClient.BroadcastTxCommit(ctx, consensusTx)
		done <- struct {
			result *cmtrpctypes.ResultBroadcastTxCommit
			err    error
		}{result, err}
	}()

	// Wait for consensus result
	select {
	case <-ctx.Done():
		return nil, &RepositoryError{
			Code:    "CONSENSUS_TIMEOUT",
			Message: "Consensus operation timed out",
			Detail:  ctx.Err().Error(),
		}
	case result := <-done:
		if result.err != nil {
			return nil, &RepositoryError{
				Code:    "CONSENSUS_ERROR",
				Message: "Failed to commit to blockchain",
				Detail:  result.err.Error(),
			}
		}

		if result.result.CheckTx.Code != 0 {
			return nil, &RepositoryError{
				Code:    "CONSENSUS_ERROR",
				Message: "Blockchain rejected transaction",
				Detail:  fmt.Sprintf("CheckTx code: %d", result.result.CheckTx.Code),
			}
		}

		return &ConsensusResult{
			TxHash:      hex.EncodeToString(result.result.Hash),
			BlockHeight: result.result.Height,
			Code:        result.result.CheckTx.Code,
		}, nil
	}
}

// Cross-Shard Query Methods

// GetSessionsByClientGroup retrieves all sessions for a client group across shards
func (r *Repository) GetSessionsByClientGroup(clientGroup string) ([]models.Session, *RepositoryError) {
	var sessions []models.Session
	err := r.db.Preload("Shard").Preload("Transaction").
		Where("client_group = ?", clientGroup).Find(&sessions).Error

	if err != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to query sessions",
			Detail:  err.Error(),
		}
	}

	return sessions, nil
}

// GetSessionsByShard retrieves all sessions from a specific shard
func (r *Repository) GetSessionsByShard(shardID string) ([]models.Session, *RepositoryError) {
	var sessions []models.Session
	err := r.db.Preload("Shard").Preload("Transaction").
		Where("shard_id = ?", shardID).Find(&sessions).Error

	if err != nil {
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to query sessions by shard",
			Detail:  err.Error(),
		}
	}

	return sessions, nil
}

// GetTransactionByHash retrieves transaction by hash (cross-shard)
func (r *Repository) GetTransactionByHash(txHash string) (*models.Transaction, *RepositoryError) {
	var transaction models.Transaction
	err := r.db.Preload("Session").Preload("Shard").
		Where("tx_hash = ?", txHash).First(&transaction).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "TRANSACTION_NOT_FOUND",
				Message: "Transaction not found",
				Detail:  fmt.Sprintf("Transaction with hash %s not found", txHash),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Failed to query transaction",
			Detail:  err.Error(),
		}
	}

	return &transaction, nil
}
