package models

import "time"

// Session represents a work session in the L2 shard
type Session struct {
	ID          string    `gorm:"column:session_id;primaryKey;type:varchar(50)"`
	OperatorID  string    `gorm:"column:operator_id;type:varchar(50);not null"`
	Status      string    `gorm:"column:status;type:varchar(20);not null"` // active, completed, committed
	IsCommitted bool      `gorm:"column:is_committed;default:false"`
	PackageID   *string   `gorm:"column:package_id;type:varchar(50)"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`

	// L1 commitment info
	L1TxHash      *string    `gorm:"column:l1_tx_hash;type:varchar(66)"`
	L1BlockHeight *int64     `gorm:"column:l1_block_height"`
	L1CommitTime  *time.Time `gorm:"column:l1_commit_time"`

	// Relationships
	Package  *Package  `gorm:"foreignKey:PackageID;references:ID"`
	QCRecord *QCRecord `gorm:"foreignKey:SessionID"`
	Label    *Label    `gorm:"foreignKey:SessionID"`
}

// Package represents a package being processed
type Package struct {
	ID         string  `gorm:"column:package_id;primaryKey;type:varchar(50)"`
	Signature  string  `gorm:"column:signature;type:varchar(255);not null"`
	SupplierID string  `gorm:"column:supplier_id;type:varchar(50);not null"`
	Status     string  `gorm:"column:status;type:varchar(20);default:'pending'"` // pending, pending_validation, validated, qc_passed, labeled
	IsTrusted  bool    `gorm:"column:is_trusted;default:false"`
	SessionID  *string `gorm:"column:session_id;type:varchar(50)"`

	// Relationships
	Supplier *Supplier `gorm:"foreignKey:SupplierID"`
	Items    []Item    `gorm:"foreignKey:PackageID"`
}

// Item represents an item in a package
type Item struct {
	ID          string `gorm:"column:item_id;primaryKey;type:varchar(50)"`
	PackageID   string `gorm:"column:package_id;type:varchar(50);not null"`
	Description string `gorm:"column:description;type:varchar(255);not null"`
	Quantity    int    `gorm:"column:quantity;not null"`
}

// Supplier represents a supplier
type Supplier struct {
	ID      string `gorm:"column:supplier_id;primaryKey;type:varchar(50)"`
	Name    string `gorm:"column:name;type:varchar(100);not null"`
	Country string `gorm:"column:country;type:varchar(50)"`
}

// QCRecord represents a quality control check
type QCRecord struct {
	ID        string    `gorm:"column:qc_id;primaryKey;type:varchar(50)"`
	SessionID string    `gorm:"column:session_id;type:varchar(50);uniqueIndex;not null"`
	Passed    bool      `gorm:"column:passed;not null"`
	Issues    string    `gorm:"column:issues;type:text"` // JSON array of issues
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// Label represents shipping label information
type Label struct {
	ID         string    `gorm:"column:label_id;primaryKey;type:varchar(50)"`
	SessionID  string    `gorm:"column:session_id;type:varchar(50);uniqueIndex;not null"`
	CourierID  string    `gorm:"column:courier_id;type:varchar(50);not null"`
	TrackingNo string    `gorm:"column:tracking_no;type:varchar(100);not null"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`

	// Relationships
	Courier *Courier `gorm:"foreignKey:CourierID"`
}

// Courier represents a shipping courier
type Courier struct {
	ID   string `gorm:"column:courier_id;primaryKey;type:varchar(50)"`
	Name string `gorm:"column:name;type:varchar(100);not null"`
}
