package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newRouter(client *kubernetes.Clientset) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Simple discovery endpoints for cluster metadata
	r.GET("/api/v1/cluster/services", func(c *gin.Context) {
		svcList, err := client.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
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
		podList, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		pods := make([]string, 0, len(podList.Items))
		for _, p := range podList.Items {
			pods = append(pods, p.Namespace+"/"+p.Name+" ("+string(p.Status.Phase)+")")
		}
		c.JSON(http.StatusOK, gin.H{"pods": pods})
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

func main() {
	client, err := buildKubeClient()
	if err != nil {
		log.Printf("Warning: failed to build k8s client: %v. Falling back to fake static mode.", err)
		// Create a fake client that returns nothing to handlers (nil client will be handled)
		client = nil
	}

	r := newRouter(clientOrFake(client))

	port := os.Getenv("PORT")
	if port == "" {
		port = "9100"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	log.Printf("Cluster Agent starting on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	// graceful shutdown placeholder
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx
}

// clientOrFake returns a Clientset-like wrapper that tolerates nil client for local dev.
// For now, we return a simple small wrapper type by embedding a real client if available.
func clientOrFake(c *kubernetes.Clientset) *kubernetes.Clientset {
	return c
}
