package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the database connection
type DB struct {
	*sql.DB
}

// InitDB initializes the database connection
func InitDB() (*DB, error) {
	dbPath := os.Getenv("RCA_DB_PATH")
	if dbPath == "" {
		dbPath = "./rca-app.db"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	wrapper := &DB{db}
	if err := wrapper.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	log.Printf("Database initialized at %s", dbPath)
	return wrapper, nil
}

// createTables creates all required tables
func (db *DB) createTables() error {
	schema := `
	-- SLO configurations
	CREATE TABLE IF NOT EXISTS slos (
		name TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		target REAL NOT NULL,
		window TEXT NOT NULL,
		indicator_query TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Alert configurations
	CREATE TABLE IF NOT EXISTS alert_configs (
		provider TEXT PRIMARY KEY,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		config TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Deployment events
	CREATE TABLE IF NOT EXISTS deployments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		namespace TEXT NOT NULL,
		status TEXT NOT NULL,
		replicas INTEGER,
		ready_replicas INTEGER,
		updated_replicas INTEGER,
		version TEXT,
		image TEXT,
		message TEXT,
		timestamp TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_deployments_namespace_name 
		ON deployments(namespace, name);
	CREATE INDEX IF NOT EXISTS idx_deployments_timestamp 
		ON deployments(timestamp DESC);

	-- Cost data
	CREATE TABLE IF NOT EXISTS costs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service TEXT NOT NULL,
		cost REAL NOT NULL,
		currency TEXT NOT NULL,
		provider TEXT NOT NULL,
		period TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		tags TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_costs_service 
		ON costs(service);
	CREATE INDEX IF NOT EXISTS idx_costs_period 
		ON costs(period);

	-- Inspection results
	CREATE TABLE IF NOT EXISTS inspection_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		application TEXT NOT NULL,
		name TEXT NOT NULL,
		category TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT NOT NULL,
		description TEXT,
		remediation TEXT,
		timestamp TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_inspection_results_app 
		ON inspection_results(application);
	CREATE INDEX IF NOT EXISTS idx_inspection_results_timestamp 
		ON inspection_results(timestamp DESC);
	`

	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
