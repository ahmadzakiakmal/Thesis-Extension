package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-2/repository/models"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// RepositoryError represents repository layer errors
type RepositoryError struct {
	Code    string
	Message string
	Detail  string
}

func (e *RepositoryError) Error() string {
	return fmt.Sprintf("%s: %s - %s", e.Code, e.Message, e.Detail)
}

// Repository handles all database operations for L2 shard
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new repository instance
func NewRepository() *Repository {
	return &Repository{}
}

// ConnectDB establishes database connection and performs migrations
func (r *Repository) ConnectDB(dsn string) error {
	for i := 0; i < 10; i++ {
		log.Printf("Database connection attempt %d...\n", i+1)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Printf("Connection attempt %d failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		r.db = db
		log.Println("✓ Connected to database")

		// Run migrations
		if err := r.Migrate(); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}

		// Seed data
		r.Seed()

		return nil
	}
	return fmt.Errorf("failed to connect to database after 10 attempts")
}

// Migrate performs database schema migrations
func (r *Repository) Migrate() error {
	log.Println("Running database migrations...")

	migrator := r.db.Migrator()

	// Order matters due to foreign keys
	tables := []interface{}{
		&models.Supplier{},
		&models.Package{},
		&models.Item{},
		&models.Session{},
		&models.QCRecord{},
		&models.Courier{},
		&models.Label{},
	}

	for _, table := range tables {
		if !migrator.HasTable(table) {
			if err := migrator.CreateTable(table); err != nil {
				return fmt.Errorf("failed to create table: %w", err)
			}
		}
	}

	log.Println("✓ Database migrations completed")
	return nil
}

// Seed initializes database with test data
func (r *Repository) Seed() {
	// Check if data already exists
	var supplierCount int64
	r.db.Model(&models.Supplier{}).Count(&supplierCount)
	if supplierCount > 0 {
		log.Println("Seed data already exists, skipping...")
		return
	}

	log.Println("Seeding database with test data...")

	// Create suppliers
	suppliers := []models.Supplier{
		{ID: "SUP-001", Name: "Acme Electronics", Country: "Japan"},
		{ID: "SUP-002", Name: "Global Tech Supply", Country: "Taiwan"},
		{ID: "SUP-003", Name: "Premium Parts Co", Country: "Germany"},
	}
	for _, supplier := range suppliers {
		r.db.Create(&supplier)
	}

	// Create couriers
	couriers := []models.Courier{
		{ID: "CUR-001", Name: "FastShip Express"},
		{ID: "CUR-002", Name: "Global Logistics"},
		{ID: "CUR-003", Name: "Quick Delivery Co"},
	}
	for _, courier := range couriers {
		r.db.Create(&courier)
	}

	// Create sample packages
	packages := []models.Package{
		{
			ID:         "PKG-001",
			Signature:  "sig_acme_electronics_001",
			SupplierID: "SUP-001",
			Status:     "pending",
		},
		{
			ID:         "PKG-002",
			Signature:  "sig_global_tech_002",
			SupplierID: "SUP-002",
			Status:     "pending",
		},
	}
	for _, pkg := range packages {
		r.db.Create(&pkg)
	}

	// Create items for packages
	items := []models.Item{
		{ID: "ITEM-001", PackageID: "PKG-001", Description: "Microcontroller Unit", Quantity: 100},
		{ID: "ITEM-002", PackageID: "PKG-001", Description: "LED Display Module", Quantity: 50},
		{ID: "ITEM-003", PackageID: "PKG-002", Description: "Power Supply Unit", Quantity: 25},
		{ID: "ITEM-004", PackageID: "PKG-002", Description: "Circuit Board", Quantity: 75},
	}
	for _, item := range items {
		r.db.Create(&item)
	}

	log.Println("✓ Database seeding completed")
}

// CreateSession creates a new session
func (r *Repository) CreateSession(operatorID string) (*models.Session, *RepositoryError) {
	sessionID := fmt.Sprintf("SES-%s", uuid.New().String()[:8])

	session := models.Session{
		ID:          sessionID,
		OperatorID:  operatorID,
		Status:      "active",
		IsCommitted: false,
	}

	if err := r.db.Create(&session).Error; err != nil {
		return nil, &RepositoryError{
			Code:    "CREATE_FAILED",
			Message: "Failed to create session",
			Detail:  err.Error(),
		}
	}

	return &session, nil
}

// GetSession retrieves a session by ID
func (r *Repository) GetSession(sessionID string) (*models.Session, *RepositoryError) {
	var session models.Session
	err := r.db.Preload("Package.Items").
		Preload("Package.Supplier").
		Preload("QCRecord").
		Preload("Label.Courier").
		Where("session_id = ?", sessionID).
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "NOT_FOUND",
				Message: "Session not found",
				Detail:  fmt.Sprintf("Session %s does not exist", sessionID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	return &session, nil
}

// ScanPackage scans a package and links it to session
func (r *Repository) ScanPackage(sessionID, packageID string) (*models.Package, *RepositoryError) {
	dbTx := r.db.Begin()

	// Find the package
	var pkg models.Package
	err := dbTx.Preload("Items").Preload("Supplier").Where("package_id = ?", packageID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "NOT_FOUND",
				Message: "Package not found",
				Detail:  fmt.Sprintf("Package %s does not exist", packageID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Update package status and link to session
	pkg.Status = "pending_validation"
	pkg.SessionID = &sessionID

	if err := dbTx.Save(&pkg).Error; err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package",
			Detail:  err.Error(),
		}
	}

	// Update session with package ID
	if err := dbTx.Model(&models.Session{}).Where("session_id = ?", sessionID).Update("package_id", packageID).Error; err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update session",
			Detail:  err.Error(),
		}
	}

	if err := dbTx.Commit().Error; err != nil {
		return nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return &pkg, nil
}

// ValidatePackage validates package signature
func (r *Repository) ValidatePackage(signature, packageID, sessionID string) (*models.Package, *RepositoryError) {
	dbTx := r.db.Begin()

	var pkg models.Package
	err := dbTx.Preload("Items").Preload("Supplier").Where("package_id = ?", packageID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "NOT_FOUND",
				Message: "Package not found",
				Detail:  fmt.Sprintf("Package %s does not exist", packageID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// For PoC, assume all signatures are valid
	pkg.IsTrusted = true
	pkg.Status = "validated"
	pkg.SessionID = &sessionID

	if err := dbTx.Save(&pkg).Error; err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package",
			Detail:  err.Error(),
		}
	}

	if err := dbTx.Commit().Error; err != nil {
		return nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return &pkg, nil
}

// QualityCheck performs quality check on package
func (r *Repository) QualityCheck(sessionID string, passed bool, issues []string) (*models.Package, *models.QCRecord, *RepositoryError) {
	dbTx := r.db.Begin()

	// Get session
	var session models.Session
	err := dbTx.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &RepositoryError{
				Code:    "NOT_FOUND",
				Message: "Session not found",
				Detail:  fmt.Sprintf("Session %s does not exist", sessionID),
			}
		}
		return nil, nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Get package
	var pkg models.Package
	err = dbTx.Preload("Items").Where("session_id = ?", sessionID).First(&pkg).Error
	if err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &RepositoryError{
				Code:    "NOT_FOUND",
				Message: "Package not found for session",
				Detail:  fmt.Sprintf("No package linked to session %s", sessionID),
			}
		}
		return nil, nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Create QC record
	issuesJSON, _ := json.Marshal(issues)
	qcRecord := models.QCRecord{
		ID:        fmt.Sprintf("QC-%s", uuid.New().String()[:8]),
		SessionID: sessionID,
		Passed:    passed,
		Issues:    string(issuesJSON),
	}

	if err := dbTx.Create(&qcRecord).Error; err != nil {
		dbTx.Rollback()
		return nil, nil, &RepositoryError{
			Code:    "CREATE_FAILED",
			Message: "Failed to create QC record",
			Detail:  err.Error(),
		}
	}

	// Update package status
	if passed {
		pkg.Status = "qc_passed"
	} else {
		pkg.Status = "qc_failed"
	}

	if err := dbTx.Save(&pkg).Error; err != nil {
		dbTx.Rollback()
		return nil, nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package",
			Detail:  err.Error(),
		}
	}

	if err := dbTx.Commit().Error; err != nil {
		return nil, nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	return &pkg, &qcRecord, nil
}

// LabelPackage creates shipping label
func (r *Repository) LabelPackage(sessionID, courierID string) (*models.Label, *RepositoryError) {
	dbTx := r.db.Begin()

	// Verify courier exists
	var courier models.Courier
	if err := dbTx.Where("courier_id = ?", courierID).First(&courier).Error; err != nil {
		dbTx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &RepositoryError{
				Code:    "NOT_FOUND",
				Message: "Courier not found",
				Detail:  fmt.Sprintf("Courier %s does not exist", courierID),
			}
		}
		return nil, &RepositoryError{
			Code:    "DATABASE_ERROR",
			Message: "Database error",
			Detail:  err.Error(),
		}
	}

	// Create label
	label := models.Label{
		ID:         fmt.Sprintf("LBL-%s", uuid.New().String()[:8]),
		SessionID:  sessionID,
		CourierID:  courierID,
		TrackingNo: fmt.Sprintf("TRK-%s", uuid.New().String()[:12]),
	}

	if err := dbTx.Create(&label).Error; err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "CREATE_FAILED",
			Message: "Failed to create label",
			Detail:  err.Error(),
		}
	}

	// Update package status
	if err := dbTx.Model(&models.Package{}).Where("session_id = ?", sessionID).Update("status", "labeled").Error; err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update package",
			Detail:  err.Error(),
		}
	}

	// Update session status to completed
	if err := dbTx.Model(&models.Session{}).Where("session_id = ?", sessionID).Update("status", "completed").Error; err != nil {
		dbTx.Rollback()
		return nil, &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to update session",
			Detail:  err.Error(),
		}
	}

	if err := dbTx.Commit().Error; err != nil {
		return nil, &RepositoryError{
			Code:    "COMMIT_FAILED",
			Message: "Failed to commit transaction",
			Detail:  err.Error(),
		}
	}

	// Reload with courier info
	dbTx = r.db.Begin()
	dbTx.Preload("Courier").Where("label_id = ?", label.ID).First(&label)
	dbTx.Commit()

	return &label, nil
}

// MarkSessionCommitted updates session with L1 commitment info
func (r *Repository) MarkSessionCommitted(sessionID, txHash string, blockHeight int64) *RepositoryError {
	commitTime := time.Now()

	err := r.db.Model(&models.Session{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"is_committed":    true,
			"status":          "committed",
			"l1_tx_hash":      txHash,
			"l1_block_height": blockHeight,
			"l1_commit_time":  commitTime,
		}).Error

	if err != nil {
		return &RepositoryError{
			Code:    "UPDATE_FAILED",
			Message: "Failed to mark session as committed",
			Detail:  err.Error(),
		}
	}

	return nil
}
