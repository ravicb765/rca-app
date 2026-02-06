package database

import (
	"database/sql"
	"encoding/json"
	"time"
)

// SLORepository handles SLO persistence
type SLORepository struct {
	db *DB
}

// NewSLORepository creates a new SLO repository
func NewSLORepository(db *DB) *SLORepository {
	return &SLORepository{db: db}
}

// SLORecord represents a stored SLO
type SLORecord struct {
	Name           string
	Type           string
	Target         float64
	Window         string
	IndicatorQuery string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Save saves an SLO configuration
func (r *SLORepository) Save(slo SLORecord) error {
	query := `
		INSERT INTO slos (name, type, target, window, indicator_query, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			type = excluded.type,
			target = excluded.target,
			window = excluded.window,
			indicator_query = excluded.indicator_query,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.db.Exec(query, slo.Name, slo.Type, slo.Target, slo.Window, slo.IndicatorQuery)
	return err
}

// Get retrieves an SLO by name
func (r *SLORepository) Get(name string) (*SLORecord, error) {
	query := `SELECT name, type, target, window, indicator_query, created_at, updated_at 
	          FROM slos WHERE name = ?`
	
	var slo SLORecord
	err := r.db.QueryRow(query, name).Scan(
		&slo.Name, &slo.Type, &slo.Target, &slo.Window, 
		&slo.IndicatorQuery, &slo.CreatedAt, &slo.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &slo, nil
}

// List retrieves all SLOs
func (r *SLORepository) List() ([]SLORecord, error) {
	query := `SELECT name, type, target, window, indicator_query, created_at, updated_at 
	          FROM slos ORDER BY name`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slos []SLORecord
	for rows.Next() {
		var slo SLORecord
		if err := rows.Scan(
			&slo.Name, &slo.Type, &slo.Target, &slo.Window,
			&slo.IndicatorQuery, &slo.CreatedAt, &slo.UpdatedAt,
		); err != nil {
			return nil, err
		}
		slos = append(slos, slo)
	}
	return slos, rows.Err()
}

// Delete deletes an SLO
func (r *SLORepository) Delete(name string) error {
	_, err := r.db.Exec("DELETE FROM slos WHERE name = ?", name)
	return err
}

// AlertConfigRepository handles alert config persistence
type AlertConfigRepository struct {
	db *DB
}

// NewAlertConfigRepository creates a new alert config repository
func NewAlertConfigRepository(db *DB) *AlertConfigRepository {
	return &AlertConfigRepository{db: db}
}

// AlertConfigRecord represents a stored alert configuration
type AlertConfigRecord struct {
	Provider  string
	Enabled   bool
	Config    string // JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Save saves an alert configuration
func (r *AlertConfigRepository) Save(provider string, enabled bool, config map[string]interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO alert_configs (provider, enabled, config, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(provider) DO UPDATE SET
			enabled = excluded.enabled,
			config = excluded.config,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = r.db.Exec(query, provider, enabled, string(configJSON))
	return err
}

// Get retrieves an alert config by provider
func (r *AlertConfigRepository) Get(provider string) (*AlertConfigRecord, error) {
	query := `SELECT provider, enabled, config, created_at, updated_at 
	          FROM alert_configs WHERE provider = ?`
	
	var record AlertConfigRecord
	err := r.db.QueryRow(query, provider).Scan(
		&record.Provider, &record.Enabled, &record.Config,
		&record.CreatedAt, &record.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// List retrieves all alert configs
func (r *AlertConfigRepository) List() ([]AlertConfigRecord, error) {
	query := `SELECT provider, enabled, config, created_at, updated_at 
	          FROM alert_configs ORDER BY provider`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []AlertConfigRecord
	for rows.Next() {
		var record AlertConfigRecord
		if err := rows.Scan(
			&record.Provider, &record.Enabled, &record.Config,
			&record.CreatedAt, &record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		configs = append(configs, record)
	}
	return configs, rows.Err()
}

// Delete deletes an alert config
func (r *AlertConfigRepository) Delete(provider string) error {
	_, err := r.db.Exec("DELETE FROM alert_configs WHERE provider = ?", provider)
	return err
}

// DeploymentRepository handles deployment event persistence
type DeploymentRepository struct {
	db *DB
}

// NewDeploymentRepository creates a new deployment repository
func NewDeploymentRepository(db *DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

// DeploymentRecord represents a stored deployment event
type DeploymentRecord struct {
	ID              int64
	Name            string
	Namespace       string
	Status          string
	Replicas        int32
	ReadyReplicas   int32
	UpdatedReplicas int32
	Version         string
	Image           string
	Message         string
	Timestamp       time.Time
	CreatedAt       time.Time
}

// Save saves a deployment event
func (r *DeploymentRepository) Save(deployment DeploymentRecord) error {
	query := `
		INSERT INTO deployments 
		(name, namespace, status, replicas, ready_replicas, updated_replicas, 
		 version, image, message, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		deployment.Name, deployment.Namespace, deployment.Status,
		deployment.Replicas, deployment.ReadyReplicas, deployment.UpdatedReplicas,
		deployment.Version, deployment.Image, deployment.Message, deployment.Timestamp,
	)
	return err
}

// GetHistory retrieves deployment history for a service
func (r *DeploymentRepository) GetHistory(namespace, name string, limit int) ([]DeploymentRecord, error) {
	query := `
		SELECT id, name, namespace, status, replicas, ready_replicas, updated_replicas,
		       version, image, message, timestamp, created_at
		FROM deployments
		WHERE namespace = ? AND name = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	
	rows, err := r.db.Query(query, namespace, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []DeploymentRecord
	for rows.Next() {
		var d DeploymentRecord
		if err := rows.Scan(
			&d.ID, &d.Name, &d.Namespace, &d.Status,
			&d.Replicas, &d.ReadyReplicas, &d.UpdatedReplicas,
			&d.Version, &d.Image, &d.Message, &d.Timestamp, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

// GetLatest retrieves the latest deployment for a service
func (r *DeploymentRepository) GetLatest(namespace, name string) (*DeploymentRecord, error) {
	query := `
		SELECT id, name, namespace, status, replicas, ready_replicas, updated_replicas,
		       version, image, message, timestamp, created_at
		FROM deployments
		WHERE namespace = ? AND name = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`
	
	var d DeploymentRecord
	err := r.db.QueryRow(query, namespace, name).Scan(
		&d.ID, &d.Name, &d.Namespace, &d.Status,
		&d.Replicas, &d.ReadyReplicas, &d.UpdatedReplicas,
		&d.Version, &d.Image, &d.Message, &d.Timestamp, &d.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CostRepository handles cost data persistence
type CostRepository struct {
	db *DB
}

// NewCostRepository creates a new cost repository
func NewCostRepository(db *DB) *CostRepository {
	return &CostRepository{db: db}
}

// CostRecord represents stored cost data
type CostRecord struct {
	ID        int64
	Service   string
	Cost      float64
	Currency  string
	Provider  string
	Period    string
	Timestamp time.Time
	Tags      string // JSON
	CreatedAt time.Time
}

// Save saves cost data
func (r *CostRepository) Save(cost CostRecord) error {
	query := `
		INSERT INTO costs (service, cost, currency, provider, period, timestamp, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		cost.Service, cost.Cost, cost.Currency, cost.Provider,
		cost.Period, cost.Timestamp, cost.Tags,
	)
	return err
}

// GetByService retrieves costs for a service
func (r *CostRepository) GetByService(service, period string) ([]CostRecord, error) {
	var query string
	var args []interface{}

	if period != "" {
		query = `SELECT id, service, cost, currency, provider, period, timestamp, tags, created_at
		         FROM costs WHERE service = ? AND period = ? ORDER BY timestamp DESC`
		args = []interface{}{service, period}
	} else {
		query = `SELECT id, service, cost, currency, provider, period, timestamp, tags, created_at
		         FROM costs WHERE service = ? ORDER BY timestamp DESC`
		args = []interface{}{service}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var costs []CostRecord
	for rows.Next() {
		var c CostRecord
		if err := rows.Scan(
			&c.ID, &c.Service, &c.Cost, &c.Currency, &c.Provider,
			&c.Period, &c.Timestamp, &c.Tags, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		costs = append(costs, c)
	}
	return costs, rows.Err()
}
