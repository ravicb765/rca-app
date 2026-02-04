package servicemap

import (
	"sync"
	"time"
)

// ServiceMap represents a dependency graph of applications
type ServiceMap struct {
	Applications map[string]*Application `json:"applications"`
	Connections  []Connection            `json:"connections"`
	LastUpdated  time.Time               `json:"last_updated"`
	mu           sync.RWMutex
}

// Application represents a service or application in the map
type Application struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type,omitempty"`
	Instances  []Instance        `json:"instances,omitempty"`
	Upstreams  []string          `json:"upstreams,omitempty"`
	Downstreams []string         `json:"downstreams,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Instance represents a running instance of an application
type Instance struct {
	ID          string            `json:"id"`
	NodeName    string            `json:"node_name,omitempty"`
	ContainerID string            `json:"container_id,omitempty"`
	PodName     string            `json:"pod_name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Connection represents an observed edge between two applications
type Connection struct {
	SourceApp   string  `json:"source_app"`
	DestApp     string  `json:"dest_app"`
	Protocol    string  `json:"protocol,omitempty"`
	RequestRate float64 `json:"request_rate,omitempty"`
	ErrorRate   float64 `json:"error_rate,omitempty"`
	Latency     float64 `json:"latency,omitempty"`
}

// NewServiceMap creates an empty ServiceMap
func NewServiceMap() *ServiceMap {
	return &ServiceMap{
		Applications: make(map[string]*Application),
		Connections:  []Connection{},
		LastUpdated:  time.Now(),
	}
}

// AddService adds or updates an application in the map
func (sm *ServiceMap) AddService(app *Application) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if existing, ok := sm.Applications[app.ID]; ok {
		// merge minimal fields
		existing.Name = app.Name
		if existing.Metadata == nil {
			existing.Metadata = app.Metadata
		}
	} else {
		sm.Applications[app.ID] = app
	}
	sm.LastUpdated = time.Now()
}

// AddConnection adds a connection and updates upstream/downstream lists
func (sm *ServiceMap) AddConnection(conn Connection) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	// add connection
	sm.Connections = append(sm.Connections, conn)
	// ensure apps exist
	if _, ok := sm.Applications[conn.SourceApp]; !ok {
		sm.Applications[conn.SourceApp] = &Application{ID: conn.SourceApp, Name: conn.SourceApp}
	}
	if _, ok := sm.Applications[conn.DestApp]; !ok {
		sm.Applications[conn.DestApp] = &Application{ID: conn.DestApp, Name: conn.DestApp}
	}
	// update upstream / downstream
	s := sm.Applications[conn.SourceApp]
	d := sm.Applications[conn.DestApp]
	// add dest to source downstreams
	if !contains(s.Downstreams, conn.DestApp) {
		s.Downstreams = append(s.Downstreams, conn.DestApp)
	}
	// add source to dest upstreams
	if !contains(d.Upstreams, conn.SourceApp) {
		d.Upstreams = append(d.Upstreams, conn.SourceApp)
	}
	sm.LastUpdated = time.Now()
}

func contains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

// HasDependency returns true if a depends on b (i.e., a -> b)
func (sm *ServiceMap) HasDependency(a, b string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if app, ok := sm.Applications[a]; ok {
		for _, d := range app.Downstreams {
			if d == b {
				return true
			}
		}
	}
	return false
}

// BuildServiceMap constructs a ServiceMap from a set of observed connections
func BuildServiceMap(conns []Connection) *ServiceMap {
	sm := NewServiceMap()
	for _, c := range conns {
		if c.SourceApp == "" || c.DestApp == "" {
			continue
		}
		// ensure basic apps
		if _, ok := sm.Applications[c.SourceApp]; !ok {
			sm.Applications[c.SourceApp] = &Application{ID: c.SourceApp, Name: c.SourceApp}
		}
		if _, ok := sm.Applications[c.DestApp]; !ok {
			sm.Applications[c.DestApp] = &Application{ID: c.DestApp, Name: c.DestApp}
		}
		sm.AddConnection(c)
	}
	return sm
}
