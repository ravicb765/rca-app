package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type IntegrationManager struct {
	client *http.Client
}

func NewIntegrationManager() *IntegrationManager {
	return &IntegrationManager{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *IntegrationManager) SendAlert(webhookURL string, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (m *IntegrationManager) NotifyPagerDuty(routingKey, summary, source string) error {
	payload := map[string]interface{}{
		"routing_key": routingKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary": summary,
			"source": source,
			"severity": "critical",
		},
	}
	// PagerDuty Events API V2
	return m.SendAlert("https://events.pagerduty.com/v2/enqueue", payload)
}
