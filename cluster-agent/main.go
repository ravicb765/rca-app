package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type CloudCost struct {
	Provider    string  `json:"provider"`
	Service     string  `json:"service"`
	DailyCost   float64 `json:"daily_cost"`
	MonthToDate float64 `json:"month_to_date"`
	Currency    string  `json:"currency"`
}

type CloudIntegration struct {
	mu    sync.RWMutex
	costs []CloudCost
}

func NewCloudIntegration() *CloudIntegration {
	return &CloudIntegration{
		costs: []CloudCost{},
	}
}

func (ci *CloudIntegration) Start(ctx context.Context) {
	go func() {
		// Initial fetch
		ci.updateCosts()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ci.updateCosts()
			}
		}
	}()
}

func (ci *CloudIntegration) updateCosts() {
	// In a real implementation, this would call AWS Cost Explorer or GCP Billing API
	// Here we simulate some data for demonstration
	simulated := []CloudCost{
		{Provider: "AWS", Service: "EC2", DailyCost: 45.20, MonthToDate: 1250.00, Currency: "USD"},
		{Provider: "AWS", Service: "RDS", DailyCost: 12.50, MonthToDate: 340.00, Currency: "USD"},
		{Provider: "AWS", Service: "S3", DailyCost: 1.10, MonthToDate: 35.00, Currency: "USD"},
	}

	ci.mu.Lock()
	ci.costs = simulated
	ci.mu.Unlock()
}

func (ci *CloudIntegration) GetCosts() []CloudCost {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	result := make([]CloudCost, len(ci.costs))
	copy(result, ci.costs)
	return result
}

type ProfileTarget struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	PodIP     string `json:"pod_ip"`
}

type ProfileScraper struct {
	client  *kubernetes.Clientset
	targets []ProfileTarget
	mu      sync.RWMutex
}

func NewProfileScraper(client *kubernetes.Clientset) *ProfileScraper {
	return &ProfileScraper{
		client:  client,
		targets: []ProfileTarget{},
	}
}

func (ps *ProfileScraper) Start(ctx context.Context) {
	go func() {
		// Initial scan
		ps.scan(ctx)

		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ps.scan(ctx)
			}
		}
	}()
}

func (ps *ProfileScraper) scan(ctx context.Context) {
	if ps.client == nil {
		return
	}
	pods, err := ps.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("ProfileScraper: failed to list pods: %v", err)
		return
	}

	var found []ProfileTarget
	for _, pod := range pods.Items {
		// Look for pods opted-in via annotation
		if pod.Status.Phase == corev1.PodRunning && pod.Annotations["rca.io/scrape-profile"] == "true" {
			// Placeholder: In a real implementation, we would fetch http://<pod-ip>:<port>/debug/pprof/profile
			log.Printf("ProfileScraper: found target %s/%s (%s)", pod.Namespace, pod.Name, pod.Status.PodIP)
			found = append(found, ProfileTarget{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				PodIP:     pod.Status.PodIP,
			})
		}
	}
	ps.mu.Lock()
	ps.targets = found
	ps.mu.Unlock()
}

func (ps *ProfileScraper) GetTargets() []ProfileTarget {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	result := make([]ProfileTarget, len(ps.targets))
	copy(result, ps.targets)
	return result
}

type DatabaseInstance struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	IP        string `json:"ip"`
}

type DatabaseDiscovery struct {
	client    *kubernetes.Clientset
	instances []DatabaseInstance
	mu        sync.RWMutex
}

func NewDatabaseDiscovery(client *kubernetes.Clientset) *DatabaseDiscovery {
	return &DatabaseDiscovery{
		client:    client,
		instances: []DatabaseInstance{},
	}
}

func (d *DatabaseDiscovery) Start(ctx context.Context) {
	go func() {
		// Initial scan
		d.scan(ctx)

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.scan(ctx)
			}
		}
	}()
}

func (d *DatabaseDiscovery) scan(ctx context.Context) {
	if d.client == nil {
		return
	}

	pods, err := d.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing pods for DB discovery: %v", err)
		return
	}

	var found []DatabaseInstance
	for _, pod := range pods.Items {
		if dbType := identifyDatabase(&pod); dbType != "" {
			found = append(found, DatabaseInstance{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Type:      dbType,
				IP:        pod.Status.PodIP,
			})
		}
	}

	d.mu.Lock()
	d.instances = found
	d.mu.Unlock()
}

func (d *DatabaseDiscovery) GetInstances() []DatabaseInstance {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]DatabaseInstance, len(d.instances))
	copy(result, d.instances)
	return result
}

func identifyDatabase(pod *corev1.Pod) string {
	// Check container images
	for _, container := range pod.Spec.Containers {
		img := strings.ToLower(container.Image)
		if strings.Contains(img, "postgres") {
			return "postgres"
		}
		if strings.Contains(img, "mysql") || strings.Contains(img, "mariadb") {
			return "mysql"
		}
		if strings.Contains(img, "mongo") {
			return "mongodb"
		}
		if strings.Contains(img, "redis") {
			return "redis"
		}
	}
	return ""
}

type ClusterAgent struct {
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	k8sClient        *kubernetes.Clientset
	informers        informers.SharedInformerFactory
	dbDiscovery      *DatabaseDiscovery
	cloudIntegration *CloudIntegration
	profileScraper   *ProfileScraper
	httpServer       *http.Server
}

func NewClusterAgent(client *kubernetes.Clientset) *ClusterAgent {
	ctx, cancel := context.WithCancel(context.Background())

	// Create shared informer factory (resync every 10 minutes)
	var factory informers.SharedInformerFactory
	if client != nil {
		factory = informers.NewSharedInformerFactory(client, 10*time.Minute)
	}

	return &ClusterAgent{
		ctx:              ctx,
		cancel:           cancel,
		k8sClient:        client,
		informers:        factory,
		dbDiscovery:      NewDatabaseDiscovery(client),
		cloudIntegration: NewCloudIntegration(),
		profileScraper:   NewProfileScraper(client),
	}
}

func (ca *ClusterAgent) Start(port string) error {
	// Start informers if client is available
	if ca.informers != nil {
		ca.informers.Start(ca.ctx.Done())
	}

	// Start database discovery
	ca.dbDiscovery.Start(ca.ctx)

	// Start cloud integration
	ca.cloudIntegration.Start(ca.ctx)

	// Start profile scraper
	ca.profileScraper.Start(ca.ctx)

	r := ca.setupRouter()
	ca.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	ca.wg.Add(1)
	go func() {
		defer ca.wg.Done()
		log.Printf("Cluster Agent starting on %s", ca.httpServer.Addr)
		if err := ca.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

func (ca *ClusterAgent) Stop() {
	log.Println("Stopping Cluster Agent...")
	ca.cancel()

	if ca.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ca.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}

	ca.wg.Wait()
	log.Println("Cluster Agent stopped")
}

func (ca *ClusterAgent) setupRouter() *gin.Engine {
	r := gin.Default()

	// Prometheus metrics (use a registry local to this router to avoid test
	// duplicate registration panics)
	reg := prometheus.NewRegistry()
	svcFetchTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_services_fetch_total",
		Help: "Total cluster services fetch attempts",
	})
	svcFetchError := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_services_fetch_errors_total",
		Help: "Total errors while fetching cluster services",
	})
	reg.MustRegister(svcFetchTotal, svcFetchError)

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", func(c *gin.Context) { c.String(http.StatusOK, "ready") })
	// Expose metrics for Prometheus using a handler bound to the registry
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	// Simple discovery endpoints for cluster metadata. If Kubernetes client is
	// not available (nil), return an empty list and 200 so compile-only checks
	// and dev workflows are tolerant.
	r.GET("/api/v1/cluster/services", func(c *gin.Context) {
		svcFetchTotal.Inc()
		if ca.k8sClient == nil {
			c.JSON(http.StatusOK, gin.H{"services": []string{}})
			return
		}
		svcList, err := ca.k8sClient.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			svcFetchError.Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		services := make([]string, 0, len(svcList.Items))
		for _, s := range svcList.Items {
			services = append(services, s.Namespace+"/"+s.Name)
		}
		c.JSON(http.StatusOK, gin.H{"services": services})
	})

	r.GET("/api/v1/cluster/pods", func(c *gin.Context) {
		svcFetchTotal.Inc()
		if ca.k8sClient == nil {
			c.JSON(http.StatusOK, gin.H{"pods": []string{}})
			return
		}
		podList, err := ca.k8sClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			svcFetchError.Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		pods := make([]PodInfo, 0, len(podList.Items))
		for _, p := range podList.Items {
			pods = append(pods, PodInfo{
				Name:      p.Name,
				Namespace: p.Namespace,
				IP:        p.Status.PodIP,
				Node:      p.Spec.NodeName,
				Phase:     string(p.Status.Phase),
			})
		}
		c.JSON(http.StatusOK, gin.H{"pods": pods})
	})

	r.GET("/api/v1/cluster/databases", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"databases": ca.dbDiscovery.GetInstances()})
	})

	r.GET("/api/v1/cluster/cloud/costs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"costs": ca.cloudIntegration.GetCosts()})
	})

	r.GET("/api/v1/cluster/profiles", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"profiles": ca.profileScraper.GetTargets()})
	})

	return r
}

func buildKubeClient() (*kubernetes.Clientset, error) {
	// Try in-cluster first
	config, err := rest.InClusterConfig()
	if err == nil {
		client, err := kubernetes.NewForConfig(config)
		if err == nil {
			return client, nil
		}
	}

	// Fall back to KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = home + "/.kube/config"
	}
	config2, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config2)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	IP        string `json:"ip"`
	Node      string `json:"node"`
	Phase     string `json:"phase"`
}

func main() {
	client, err := buildKubeClient()
	if err != nil {
		log.Printf("Warning: failed to build k8s client: %v. Falling back to fake static mode.", err)
		// Create a fake client that returns nothing to handlers (nil client will be handled)
		client = nil
	}

	agent := NewClusterAgent(clientOrFake(client))

	port := os.Getenv("PORT")
	if port == "" {
		port = "9100"
	}

	if err := agent.Start(port); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Wait for signals to gracefully exit
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	agent.Stop()
}

// clientOrFake returns a Clientset-like wrapper that tolerates nil client for local dev.
// For now, we return a simple small wrapper type by embedding a real client if available.
func clientOrFake(c *kubernetes.Clientset) *kubernetes.Clientset {
	return c
}
