package deployment

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	DeploymentStatusProgressing DeploymentStatus = "progressing"
	DeploymentStatusComplete    DeploymentStatus = "complete"
	DeploymentStatusFailed      DeploymentStatus = "failed"
)

// DeploymentEvent represents a deployment event
type DeploymentEvent struct {
	Name            string           `json:"name"`
	Namespace       string           `json:"namespace"`
	Status          DeploymentStatus `json:"status"`
	Replicas        int32            `json:"replicas"`
	ReadyReplicas   int32            `json:"ready_replicas"`
	UpdatedReplicas int32            `json:"updated_replicas"`
	Version         string           `json:"version,omitempty"`
	Image           string           `json:"image,omitempty"`
	Timestamp       time.Time        `json:"timestamp"`
	Message         string           `json:"message,omitempty"`
}

// DeploymentTracker watches Kubernetes deployments
type DeploymentTracker struct {
	clientset *kubernetes.Clientset
	history   map[string][]DeploymentEvent
	mu        sync.RWMutex
	stopCh    chan struct{}
	maxHistory int
}

// NewDeploymentTracker creates a new deployment tracker
func NewDeploymentTracker() (*DeploymentTracker, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &DeploymentTracker{
		clientset:  clientset,
		history:    make(map[string][]DeploymentEvent),
		stopCh:     make(chan struct{}),
		maxHistory: 100,
	}, nil
}

// Start begins watching deployments
func (dt *DeploymentTracker) Start(ctx context.Context) error {
	watcher, err := dt.clientset.AppsV1().Deployments("").Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start deployment watcher: %w", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					log.Println("Deployment watcher channel closed, restarting...")
					return
				}
				dt.handleEvent(event)
			case <-dt.stopCh:
				log.Println("Stopping deployment watcher")
				return
			case <-ctx.Done():
				log.Println("Context cancelled, stopping deployment watcher")
				return
			}
		}
	}()

	return nil
}

// Stop stops the deployment watcher
func (dt *DeploymentTracker) Stop() {
	close(dt.stopCh)
}

// handleEvent processes a deployment event
func (dt *DeploymentTracker) handleEvent(event watch.Event) {
	deployment, ok := event.Object.(*appsv1.Deployment)
	if !ok {
		return
	}

	status := DeploymentStatusProgressing
	message := ""

	// Determine deployment status
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing {
			if condition.Status == "True" && condition.Reason == "NewReplicaSetAvailable" {
				status = DeploymentStatusComplete
				message = "Deployment completed successfully"
			} else if condition.Status == "False" {
				status = DeploymentStatusFailed
				message = condition.Message
			}
		}
	}

	// Extract version and image
	version := deployment.Labels["version"]
	image := ""
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
	}

	deploymentEvent := DeploymentEvent{
		Name:            deployment.Name,
		Namespace:       deployment.Namespace,
		Status:          status,
		Replicas:        deployment.Status.Replicas,
		ReadyReplicas:   deployment.Status.ReadyReplicas,
		UpdatedReplicas: deployment.Status.UpdatedReplicas,
		Version:         version,
		Image:           image,
		Timestamp:       time.Now(),
		Message:         message,
	}

	dt.addEvent(deploymentEvent)
}

// addEvent adds a deployment event to history
func (dt *DeploymentTracker) addEvent(event DeploymentEvent) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	key := fmt.Sprintf("%s/%s", event.Namespace, event.Name)
	dt.history[key] = append(dt.history[key], event)

	// Prune history if it exceeds max
	if len(dt.history[key]) > dt.maxHistory {
		dt.history[key] = dt.history[key][len(dt.history[key])-dt.maxHistory:]
	}
}

// GetDeploymentHistory returns deployment history for a service
func (dt *DeploymentTracker) GetDeploymentHistory(namespace, name string, limit int) []DeploymentEvent {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	events := dt.history[key]

	if limit > 0 && len(events) > limit {
		return events[len(events)-limit:]
	}

	return events
}

// GetLatestDeployment returns the latest deployment event for a service
func (dt *DeploymentTracker) GetLatestDeployment(namespace, name string) *DeploymentEvent {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	events := dt.history[key]

	if len(events) == 0 {
		return nil
	}

	return &events[len(events)-1]
}

// GetAllDeployments returns all recent deployments across all services
func (dt *DeploymentTracker) GetAllDeployments(limit int) []DeploymentEvent {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	var allEvents []DeploymentEvent
	for _, events := range dt.history {
		allEvents = append(allEvents, events...)
	}

	// Sort by timestamp (most recent first)
	for i := 0; i < len(allEvents)-1; i++ {
		for j := i + 1; j < len(allEvents); j++ {
			if allEvents[i].Timestamp.Before(allEvents[j].Timestamp) {
				allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
			}
		}
	}

	if limit > 0 && len(allEvents) > limit {
		return allEvents[:limit]
	}

	return allEvents
}
