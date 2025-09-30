package models

import "time"

// ShardInfo represents information about L2 shards
type ShardInfo struct {
	ShardID     string    `gorm:"column:shard_id;primaryKey;type:varchar(50)"`
	ClientGroup string    `gorm:"column:client_group;type:varchar(100);not null"`
	L2NodeID    string    `gorm:"column:l2_node_id;type:varchar(50);not null"`
	Status      string    `gorm:"column:status;type:varchar(20);default:'active'"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`

	// NOTE: We don't define parent -> child relationships to avoid circular dependencies during migration
}

// Session represents a session from any L2 shard
type Session struct {
	ID          string     `gorm:"column:session_id;primaryKey;type:varchar(50)"`
	ShardID     string     `gorm:"column:shard_id;type:varchar(50);index;not null"`
	Shard       *ShardInfo `gorm:"foreignKey:ShardID;references:ShardID"`
	ClientGroup string     `gorm:"column:client_group;type:varchar(100);not null"`
	OperatorID  string     `gorm:"column:operator_id;type:varchar(50)"`
	Status      string     `gorm:"column:status;type:varchar(20);not null"`
	IsCommitted bool       `gorm:"column:is_committed;default:false"`
	TxHash      *string    `gorm:"column:tx_hash;type:varchar(66)"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`

	// Session data as JSON (from L2)
	SessionData string `gorm:"column:session_data;type:jsonb"`

	// Relationships
	Transaction *Transaction `gorm:"foreignKey:SessionID"`
}

// Transaction represents blockchain records with shard tracking
type Transaction struct {
	TxHash      string     `gorm:"column:tx_hash;type:varchar(66)"`
	SessionID   string     `gorm:"column:session_id;type:varchar(50);uniqueIndex;not null;primaryKey"`
	ShardID     string     `gorm:"column:shard_id;type:varchar(50);index;not null"`
	Shard       *ShardInfo `gorm:"foreignKey:ShardID;references:ShardID"`
	ClientGroup string     `gorm:"column:client_group;type:varchar(100);not null"`
	BlockHeight int64      `gorm:"column:block_height;not null"`
	Timestamp   time.Time  `gorm:"column:timestamp;not null"`
	Status      string     `gorm:"column:status;type:varchar(20);default:'confirmed'"`

	// Relationships
	Session *Session `gorm:"foreignKey:SessionID"`
}

// Operator represents users across all shards (for cross-shard queries)
type Operator struct {
	ID          string     `gorm:"column:operator_id;primaryKey;type:varchar(50)"`
	Name        string     `gorm:"column:name;type:varchar(100);not null"`
	Role        string     `gorm:"column:role;type:varchar(50)"`
	AccessLevel string     `gorm:"column:access_level;type:varchar(20);default:'Basic'"`
	ShardID     string     `gorm:"column:shard_id;type:varchar(50);index"`
	Shard       *ShardInfo `gorm:"foreignKey:ShardID;references:ShardID"`
}
