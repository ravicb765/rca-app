package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/smtp"
	"sync"
	"time"
	
	"github.com/ravicb765/rca-app/server/database"
)

// AlertSeverity represents the severity of an alert
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

// Alert represents an alert to be sent
type Alert struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Severity    AlertSeverity `json:"severity"`
	Source      string        `json:"source"`
	Timestamp   time.Time     `json:"timestamp"`
	Tags        []string      `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AlertProvider is the interface for all alert providers
type AlertProvider interface {
	Send(alert Alert) error
	Name() string
}

// AlertConfig represents the configuration for an alert provider
type AlertConfig struct {
	Provider string                 `json:"provider"`
	Enabled  bool                   `json:"enabled"`
	Config   map[string]interface{} `json:"config"`
}

// AlertManager manages alert routing and delivery
type AlertManager struct {
	providers map[string]AlertProvider
	configs   map[string]AlertConfig
	repo      *database.AlertConfigRepository
	mu        sync.RWMutex
	client    *http.Client
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{
		providers: make(map[string]AlertProvider),
		configs:   make(map[string]AlertConfig),
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// SetDatabase sets the database repository for persistence
func (am *AlertManager) SetDatabase(db *database.DB) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.repo = database.NewAlertConfigRepository(db)
	
	// Load existing configs from database
	records, err := am.repo.List()
	if err != nil {
		return fmt.Errorf("failed to load alert configs from database: %w", err)
	}
	
	for _, record := range records {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(record.Config), &config); err != nil {
			continue // Skip invalid records
		}
		
		am.configs[record.Provider] = AlertConfig{
			Provider: record.Provider,
			Enabled:  record.Enabled,
			Config:   config,
		}
	}
	
	return nil
}

// RegisterProvider registers an alert provider
func (am *AlertManager) RegisterProvider(provider AlertProvider) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.providers[provider.Name()] = provider
}

// ConfigureProvider configures an alert provider
func (am *AlertManager) ConfigureProvider(config AlertConfig) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.providers[config.Provider]; !exists {
		return fmt.Errorf("provider %s not registered", config.Provider)
	}

	am.configs[config.Provider] = config
	
	// Persist to database if available
	if am.repo != nil {
		if err := am.repo.Save(config.Provider, config.Enabled, config.Config); err != nil {
			return fmt.Errorf("failed to persist alert config: %w", err)
		}
	}
	
	return nil
}

// SendAlert sends an alert to all enabled providers
func (am *AlertManager) SendAlert(alert Alert) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var errors []string
	for name, config := range am.configs {
		if !config.Enabled {
			continue
		}

		provider, exists := am.providers[name]
		if !exists {
			continue
		}

		if err := provider.Send(alert); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("alert delivery failed: %v", errors)
	}

	return nil
}

// GetConfigs returns all alert configurations
func (am *AlertManager) GetConfigs() []AlertConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()

	configs := make([]AlertConfig, 0, len(am.configs))
	for _, config := range am.configs {
		configs = append(configs, config)
	}
	return configs
}

// RemoveProvider removes a provider configuration
func (am *AlertManager) RemoveProvider(name string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.configs[name]; !exists {
		return fmt.Errorf("provider %s not configured", name)
	}

	delete(am.configs, name)
	
	// Remove from database if available
	if am.repo != nil {
		if err := am.repo.Delete(name); err != nil {
			return fmt.Errorf("failed to delete alert config from database: %w", err)
		}
	}
	
	return nil
}

// EmailProvider sends alerts via SMTP
type EmailProvider struct {
	smtpHost string
	smtpPort string
	from     string
	to       []string
	username string
	password string
}

func NewEmailProvider(smtpHost, smtpPort, from string, to []string, username, password string) *EmailProvider {
	return &EmailProvider{
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		to:       to,
		username: username,
		password: password,
	}
}

func (e *EmailProvider) Name() string {
	return "email"
}

func (e *EmailProvider) Send(alert Alert) error {
	// Sanitize inputs
	subject := fmt.Sprintf("[%s] %s", alert.Severity, html.EscapeString(alert.Title))
	body := fmt.Sprintf("Description: %s\nSource: %s\nTimestamp: %s\n", 
		html.EscapeString(alert.Description), 
		html.EscapeString(alert.Source), 
		alert.Timestamp.Format(time.RFC3339))

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", 
		e.to[0], subject, body))

	auth := smtp.PlainAuth("", e.username, e.password, e.smtpHost)
	addr := fmt.Sprintf("%s:%s", e.smtpHost, e.smtpPort)

	return smtp.SendMail(addr, auth, e.from, e.to, msg)
}

// SlackProvider sends alerts to Slack
type SlackProvider struct {
	webhookURL string
	client     *http.Client
}

func NewSlackProvider(webhookURL string) *SlackProvider {
	return &SlackProvider{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackProvider) Name() string {
	return "slack"
}

func (s *SlackProvider) Send(alert Alert) error {
	color := "warning"
	if alert.Severity == AlertSeverityCritical {
		color = "danger"
	} else if alert.Severity == AlertSeverityInfo {
		color = "good"
	}

	// Sanitize inputs
	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color":     color,
				"title":     html.EscapeString(alert.Title),
				"text":      html.EscapeString(alert.Description),
				"footer":    html.EscapeString(alert.Source),
				"ts":        alert.Timestamp.Unix(),
				"fields": []map[string]interface{}{
					{
						"title": "Severity",
						"value": string(alert.Severity),
						"short": true,
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

// PagerDutyProvider sends alerts to PagerDuty
type PagerDutyProvider struct {
	routingKey string
	client     *http.Client
}

func NewPagerDutyProvider(routingKey string) *PagerDutyProvider {
	return &PagerDutyProvider{
		routingKey: routingKey,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *PagerDutyProvider) Name() string {
	return "pagerduty"
}

func (p *PagerDutyProvider) Send(alert Alert) error {
	severity := "error"
	if alert.Severity == AlertSeverityCritical {
		severity = "critical"
	} else if alert.Severity == AlertSeverityWarning {
		severity = "warning"
	} else {
		severity = "info"
	}

	// Sanitize inputs
	payload := map[string]interface{}{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":   html.EscapeString(alert.Title),
			"source":    html.EscapeString(alert.Source),
			"severity":  severity,
			"timestamp": alert.Timestamp.Format(time.RFC3339),
			"custom_details": map[string]interface{}{
				"description": html.EscapeString(alert.Description),
				"tags":        alert.Tags,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := p.client.Post("https://events.pagerduty.com/v2/enqueue", 
		"application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("pagerduty API failed with status: %d", resp.StatusCode)
	}

	return nil
}

// WebhookProvider sends alerts to a generic webhook
type WebhookProvider struct {
	url    string
	client *http.Client
}

func NewWebhookProvider(url string) *WebhookProvider {
	return &WebhookProvider{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookProvider) Name() string {
	return "webhook"
}

func (w *WebhookProvider) Send(alert Alert) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	resp, err := w.client.Post(w.url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

// JiraProvider sends alerts to Jira Service Management
type JiraProvider struct {
	url      string
	username string
	apiToken string
	project  string
	client   *http.Client
}

func NewJiraProvider(url, username, apiToken, project string) *JiraProvider {
	return &JiraProvider{
		url:      url,
		username: username,
		apiToken: apiToken,
		project:  project,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (j *JiraProvider) Name() string {
	return "jira"
}

func (j *JiraProvider) Send(alert Alert) error {
	issueType := "Task"
	if alert.Severity == AlertSeverityCritical {
		issueType = "Bug"
	}

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]string{
				"key": j.project,
			},
			"summary":     alert.Title,
			"description": alert.Description,
			"issuetype": map[string]string{
				"name": issueType,
			},
			"labels": alert.Tags,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/rest/api/2/issue", j.url), 
		bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.SetBasicAuth(j.username, j.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("jira API failed with status: %d", resp.StatusCode)
	}

	return nil
}

// OpsGenieProvider sends alerts to OpsGenie
type OpsGenieProvider struct {
	apiKey string
	client *http.Client
}

func NewOpsGenieProvider(apiKey string) *OpsGenieProvider {
	return &OpsGenieProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (o *OpsGenieProvider) Name() string {
	return "opsgenie"
}

func (o *OpsGenieProvider) Send(alert Alert) error {
	priority := "P3"
	if alert.Severity == AlertSeverityCritical {
		priority = "P1"
	} else if alert.Severity == AlertSeverityWarning {
		priority = "P2"
	}

	payload := map[string]interface{}{
		"message":     alert.Title,
		"description": alert.Description,
		"priority":    priority,
		"source":      alert.Source,
		"tags":        alert.Tags,
		"details":     alert.Metadata,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.opsgenie.com/v2/alerts", 
		bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("GenieKey %s", o.apiKey))

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("opsgenie API failed with status: %d", resp.StatusCode)
	}

	return nil
}

// TeamsProvider sends alerts to Microsoft Teams
type TeamsProvider struct {
	webhookURL string
	client     *http.Client
}

func NewTeamsProvider(webhookURL string) *TeamsProvider {
	return &TeamsProvider{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TeamsProvider) Name() string {
	return "teams"
}

func (t *TeamsProvider) Send(alert Alert) error {
	color := "FFA500" // Orange for warning
	if alert.Severity == AlertSeverityCritical {
		color = "FF0000" // Red
	} else if alert.Severity == AlertSeverityInfo {
		color = "00FF00" // Green
	}

	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"summary":    alert.Title,
		"themeColor": color,
		"title":      alert.Title,
		"sections": []map[string]interface{}{
			{
				"activityTitle":    alert.Source,
				"activitySubtitle": alert.Timestamp.Format(time.RFC3339),
				"facts": []map[string]string{
					{
						"name":  "Severity",
						"value": string(alert.Severity),
					},
					{
						"name":  "Description",
						"value": alert.Description,
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := t.client.Post(t.webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("teams webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}
