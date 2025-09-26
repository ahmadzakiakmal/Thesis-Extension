package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/ahmadzakiakmal/thesis-extension/layer-1/repository"
	"github.com/ahmadzakiakmal/thesis-extension/layer-1/srvreg"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/dgraph-io/badger/v4"
)

// Application implements the ABCI interface for L1 BFT consensus
type Application struct {
	badgerDB        *badger.DB
	onGoingBlock    *badger.Txn
	serviceRegistry *srvreg.ServiceRegistry
	nodeID          string
	mu              sync.Mutex
	config          *AppConfig
	logger          cmtlog.Logger
	repository      *repository.Repository
}

// AppConfig contains configuration for the L1 application
type AppConfig struct {
	NodeID        string
	RequiredVotes int
	LogAllTxs     bool
}

// NewABCIApplication creates a new L1 ABCI application
func NewABCIApplication(badgerDB *badger.DB, serviceRegistry *srvreg.ServiceRegistry, config *AppConfig, logger cmtlog.Logger, repository *repository.Repository) *Application {
	return &Application{
		badgerDB:        badgerDB,
		serviceRegistry: serviceRegistry,
		nodeID:          "",
		config:          config,
		logger:          logger,
		repository:      repository,
	}
}

func (app *Application) SetNodeID(id string) {
	app.nodeID = id
}

// Info implements the ABCI Info method
func (app *Application) Info(_ context.Context, info *abcitypes.InfoRequest) (*abcitypes.InfoResponse, error) {
	lastBlockHeight := int64(0)
	var lastBlockAppHash []byte

	err := app.badgerDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("last_block_height"))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			return err
		}

		err = item.Value(func(val []byte) error {
			lastBlockHeight = bytesToInt64(val)
			return nil
		})
		if err != nil {
			return err
		}

		item, err = txn.Get([]byte("last_block_app_hash"))
		if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		if err == nil {
			err = item.Value(func(val []byte) error {
				lastBlockAppHash = val
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error getting last block info: %v", err)
	}

	return &abcitypes.InfoResponse{
		LastBlockHeight:  lastBlockHeight,
		LastBlockAppHash: lastBlockAppHash,
	}, nil
}

// Query implements the ABCI Query method for cross-shard queries
func (app *Application) Query(_ context.Context, req *abcitypes.QueryRequest) (*abcitypes.QueryResponse, error) {
	if len(req.Data) == 0 {
		return &abcitypes.QueryResponse{
			Code: 1,
			Log:  "Empty query data",
		}, nil
	}

	// Handle verification queries
	if string(req.Data[:7]) == "verify:" {
		txID := req.Data[7:]
		return app.verifyTransaction(txID)
	}

	// Handle shard queries
	if string(req.Data[:6]) == "shard:" {
		shardID := string(req.Data[6:])
		return app.queryShardData(shardID)
	}

	// Handle regular key-value lookup
	resp := abcitypes.QueryResponse{Key: req.Data}

	dbErr := app.badgerDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(req.Data)
		if err != nil {
			if !errors.Is(err, badger.ErrKeyNotFound) {
				return err
			}
			resp.Log = "key doesn't exist"
			return nil
		}

		return item.Value(func(val []byte) error {
			resp.Log = "exists"
			resp.Value = val
			return nil
		})
	})

	if dbErr != nil {
		log.Printf("Error reading database: %v", dbErr)
		return &abcitypes.QueryResponse{
			Code: 2,
			Log:  fmt.Sprintf("Database error: %v", dbErr),
		}, nil
	}

	return &resp, nil
}

// verifyTransaction verifies a cross-shard transaction
func (app *Application) verifyTransaction(txID []byte) (*abcitypes.QueryResponse, error) {
	var resp abcitypes.QueryResponse

	err := app.badgerDB.View(func(txn *badger.Txn) error {
		txKey := append([]byte("tx:"), txID...)
		item, err := txn.Get(txKey)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				resp.Log = "Transaction not found"
				resp.Code = 1
				return nil
			}
			return err
		}

		var txData []byte
		err = item.Value(func(val []byte) error {
			txData = append([]byte{}, val...)
			return nil
		})
		if err != nil {
			return err
		}

		// Get status
		statusKey := append([]byte("status:"), txID...)
		item, err = txn.Get(statusKey)
		status := "confirmed"
		if err == nil {
			err = item.Value(func(val []byte) error {
				status = string(val)
				return nil
			})
			if err != nil {
				return err
			}
		}

		resp.Value = txData
		resp.Log = status
		resp.Code = 0
		return nil
	})

	if err != nil {
		resp.Code = 2
		resp.Log = fmt.Sprintf("Database error: %v", err)
	}

	return &resp, nil
}

// queryShardData queries data from a specific shard
func (app *Application) queryShardData(shardID string) (*abcitypes.QueryResponse, error) {
	var resp abcitypes.QueryResponse

	err := app.badgerDB.View(func(txn *badger.Txn) error {
		shardKey := append([]byte("shard:"), []byte(shardID)...)
		item, err := txn.Get(shardKey)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				resp.Log = "Shard not found"
				resp.Code = 1
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			resp.Value = val
			resp.Log = "found"
			resp.Code = 0
			return nil
		})
	})

	if err != nil {
		resp.Code = 2
		resp.Log = fmt.Sprintf("Database error: %v", err)
	}

	return &resp, nil
}

// CheckTx implements the ABCI CheckTx method
func (app *Application) CheckTx(_ context.Context, check *abcitypes.CheckTxRequest) (*abcitypes.CheckTxResponse, error) {
	txBytes := check.Tx

	// Try to parse as shard commit request
	var shardCommit repository.ShardedCommitRequest
	err := json.Unmarshal(txBytes, &shardCommit)
	if err != nil {
		return &abcitypes.CheckTxResponse{Code: 1},
			fmt.Errorf("malformed shard commit transaction: %s", err.Error())
	}

	// Validate required fields
	if shardCommit.ShardID == "" || shardCommit.SessionID == "" || shardCommit.ClientGroup == "" {
		return &abcitypes.CheckTxResponse{Code: 1},
			fmt.Errorf("missing required fields in shard commit")
	}

	return &abcitypes.CheckTxResponse{Code: 0}, nil
}

// InitChain implements the ABCI InitChain method
func (app *Application) InitChain(_ context.Context, chain *abcitypes.InitChainRequest) (*abcitypes.InitChainResponse, error) {
	return &abcitypes.InitChainResponse{}, nil
}

// PrepareProposal implements the ABCI PrepareProposal method
func (app *Application) PrepareProposal(_ context.Context, proposal *abcitypes.PrepareProposalRequest) (*abcitypes.PrepareProposalResponse, error) {
	return &abcitypes.PrepareProposalResponse{Txs: proposal.Txs}, nil
}

// ProcessProposal implements the ABCI ProcessProposal method
func (app *Application) ProcessProposal(_ context.Context, proposal *abcitypes.ProcessProposalRequest) (*abcitypes.ProcessProposalResponse, error) {
	app.logger.Info("Processing proposal with transactions", "count", len(proposal.Txs))

	for i, txBytes := range proposal.Txs {
		var shardCommit repository.ShardedCommitRequest
		err := json.Unmarshal(txBytes, &shardCommit)
		if err != nil {
			app.logger.Error("Invalid transaction format", "index", i, "error", err)
			return &abcitypes.ProcessProposalResponse{
				Status: abcitypes.PROCESS_PROPOSAL_STATUS_REJECT,
			}, fmt.Errorf("invalid transaction at index %d: %v", i, err)
		}

		// Validate shard commit structure
		if shardCommit.ShardID == "" || shardCommit.SessionID == "" {
			app.logger.Error("Invalid shard commit", "index", i, "shard_id", shardCommit.ShardID, "session_id", shardCommit.SessionID)
			return &abcitypes.ProcessProposalResponse{
				Status: abcitypes.PROCESS_PROPOSAL_STATUS_REJECT,
			}, fmt.Errorf("invalid shard commit at index %d", i)
		}

		app.logger.Info("Validating shard commit", "index", i, "shard_id", shardCommit.ShardID, "session_id", shardCommit.SessionID)
	}

	return &abcitypes.ProcessProposalResponse{
		Status: abcitypes.PROCESS_PROPOSAL_STATUS_ACCEPT,
	}, nil
}

// FinalizeBlock implements the ABCI FinalizeBlock method
func (app *Application) FinalizeBlock(_ context.Context, req *abcitypes.FinalizeBlockRequest) (*abcitypes.FinalizeBlockResponse, error) {
	var txResults = make([]*abcitypes.ExecTxResult, len(req.Txs))

	app.mu.Lock()
	defer app.mu.Unlock()

	app.onGoingBlock = app.badgerDB.NewTransaction(true)

	for i, txBytes := range req.Txs {
		var shardCommit repository.ShardedCommitRequest
		if err := json.Unmarshal(txBytes, &shardCommit); err != nil {
			txResults[i] = &abcitypes.ExecTxResult{
				Code: 1,
				Log:  "Invalid shard commit format",
			}
			continue
		}

		txID := generateTxID(shardCommit.SessionID, shardCommit.ShardID)
		txResults[i] = app.storeShardCommit(txID, &shardCommit, "accepted", txBytes)
	}

	// Store block info
	blockHeight := req.Height
	appHash := calculateAppHash(txResults)

	err := app.onGoingBlock.Set([]byte("last_block_height"), int64ToBytes(blockHeight))
	if err != nil {
		log.Printf("Error storing block height: %v", err)
	}

	err = app.onGoingBlock.Set([]byte("last_block_app_hash"), appHash)
	if err != nil {
		log.Printf("Error storing app hash: %v", err)
	}

	return &abcitypes.FinalizeBlockResponse{
		TxResults: txResults,
		AppHash:   appHash,
	}, nil
}

// storeShardCommit stores the shard commit in the database
func (app *Application) storeShardCommit(txID string, shardCommit *repository.ShardedCommitRequest, status string, rawTx []byte) *abcitypes.ExecTxResult {
	// Store the transaction
	txKey := append([]byte("tx:"), []byte(txID)...)
	err := app.onGoingBlock.Set(txKey, rawTx)
	if err != nil {
		log.Printf("Error storing transaction: %v", err)
		return &abcitypes.ExecTxResult{
			Code: 3,
			Log:  fmt.Sprintf("Database error: %v", err),
		}
	}

	// Store by shard
	shardKey := fmt.Sprintf("shard:%s:session:%s", shardCommit.ShardID, shardCommit.SessionID)
	err = app.onGoingBlock.Set([]byte(shardKey), rawTx)
	if err != nil {
		log.Printf("Error storing shard data: %v", err)
	}

	// Store status
	statusKey := append([]byte("status:"), []byte(txID)...)
	err = app.onGoingBlock.Set(statusKey, []byte(status))
	if err != nil {
		log.Printf("Error storing transaction status: %v", err)
	}

	// Create events
	events := []abcitypes.Event{
		{
			Type: "l1_shard_commit",
			Attributes: []abcitypes.EventAttribute{
				{Key: "session_id", Value: shardCommit.SessionID, Index: true},
				{Key: "shard_id", Value: shardCommit.ShardID, Index: true},
				{Key: "client_group", Value: shardCommit.ClientGroup, Index: true},
				{Key: "tx_id", Value: txID, Index: true},
				{Key: "status", Value: status, Index: true},
			},
		},
	}

	return &abcitypes.ExecTxResult{
		Code:   0,
		Data:   []byte(txID),
		Log:    status,
		Events: events,
	}
}

// Commit implements the ABCI Commit method
func (app *Application) Commit(_ context.Context, commit *abcitypes.CommitRequest) (*abcitypes.CommitResponse, error) {
	err := app.onGoingBlock.Commit()
	if err != nil {
		log.Printf("Error committing block: %v", err)
	}
	return &abcitypes.CommitResponse{}, nil
}

// Placeholder implementations for other ABCI methods
func (app *Application) ListSnapshots(_ context.Context, snapshots *abcitypes.ListSnapshotsRequest) (*abcitypes.ListSnapshotsResponse, error) {
	return &abcitypes.ListSnapshotsResponse{}, nil
}

func (app *Application) OfferSnapshot(_ context.Context, snapshot *abcitypes.OfferSnapshotRequest) (*abcitypes.OfferSnapshotResponse, error) {
	return &abcitypes.OfferSnapshotResponse{}, nil
}

func (app *Application) LoadSnapshotChunk(_ context.Context, chunk *abcitypes.LoadSnapshotChunkRequest) (*abcitypes.LoadSnapshotChunkResponse, error) {
	return &abcitypes.LoadSnapshotChunkResponse{}, nil
}

func (app *Application) ApplySnapshotChunk(_ context.Context, chunk *abcitypes.ApplySnapshotChunkRequest) (*abcitypes.ApplySnapshotChunkResponse, error) {
	return &abcitypes.ApplySnapshotChunkResponse{
		Result: abcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ACCEPT,
	}, nil
}

func (app *Application) ExtendVote(_ context.Context, extend *abcitypes.ExtendVoteRequest) (*abcitypes.ExtendVoteResponse, error) {
	return &abcitypes.ExtendVoteResponse{}, nil
}

func (app *Application) VerifyVoteExtension(_ context.Context, verify *abcitypes.VerifyVoteExtensionRequest) (*abcitypes.VerifyVoteExtensionResponse, error) {
	return &abcitypes.VerifyVoteExtensionResponse{}, nil
}

// Helper functions

// generateTxID generates a unique ID for a shard commit transaction
func generateTxID(sessionID, shardID string) string {
	hash := sha256.Sum256([]byte(sessionID + shardID))
	return hex.EncodeToString(hash[:])
}

// calculateAppHash calculates the application hash for the current block
func calculateAppHash(txResults []*abcitypes.ExecTxResult) []byte {
	allData := make([]byte, 0)
	for _, result := range txResults {
		allData = append(allData, result.Data...)
	}
	hash := sha256.Sum256(allData)
	return hash[:]
}

// int64ToBytes converts an int64 to bytes
func int64ToBytes(i int64) []byte {
	buf := make([]byte, 8)
	buf[0] = byte(i >> 56)
	buf[1] = byte(i >> 48)
	buf[2] = byte(i >> 40)
	buf[3] = byte(i >> 32)
	buf[4] = byte(i >> 24)
	buf[5] = byte(i >> 16)
	buf[6] = byte(i >> 8)
	buf[7] = byte(i)
	return buf
}

// bytesToInt64 converts bytes to an int64
func bytesToInt64(buf []byte) int64 {
	if len(buf) < 8 {
		return 0
	}
	return int64(buf[0])<<56 |
		int64(buf[1])<<48 |
		int64(buf[2])<<40 |
		int64(buf[3])<<32 |
		int64(buf[4])<<24 |
		int64(buf[5])<<16 |
		int64(buf[6])<<8 |
		int64(buf[7])
}
