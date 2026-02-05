package logclus

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Simplified Drain algorithm implementation
// 1. Group by log message length
// 2. Tokenize and match pattern

type LogNode struct {
	Key      string
	Children map[string]*LogNode
	IsLeaf   bool
	ClusterID string
}

type Drain struct {
	root        *LogNode
	clusters    map[string]*LogCluster
	depth       int
	simThresh   float64
	clusterCount int
	mu          sync.Mutex
}

type LogCluster struct {
	ID          string
	Pattern     string
	SampleLogs  []string
	Count       int
}

func NewDrain() *Drain {
	return &Drain{
		root: &LogNode{Children: make(map[string]*LogNode)},
		clusters: make(map[string]*LogCluster),
		depth: 4,
		simThresh: 0.4,
	}
}

func (d *Drain) Train(logMsg string) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Preprocessing: Remove digits/vars for better matching
	// regex to mask digts
	re := regexp.MustCompile(`\d+`)
	masked := re.ReplaceAllString(logMsg, "<*>)") // Basic masking
	
	tokens := strings.Fields(masked)
	if len(tokens) == 0 {
		return ""
	}

	// Traverse tree
	curr := d.root
	matchLength := len(tokens)
	if matchLength > d.depth {
		matchLength = d.depth
	}

	// 1. Length Layer (Simple map key in root logic)
	lenKey := fmt.Sprintf("%d", len(tokens))
	if _, ok := curr.Children[lenKey]; !ok {
		curr.Children[lenKey] = &LogNode{Key: lenKey, Children: make(map[string]*LogNode)}
	}
	curr = curr.Children[lenKey]

	// 2. Token Layers
	for i := 0; i < matchLength; i++ {
		token := tokens[i]
		if _, ok := curr.Children[token]; !ok {
			// If no exact match, try wildcard
			if _, okWC := curr.Children["<*>"]; !okWC {
				curr.Children[token] = &LogNode{Key: token, Children: make(map[string]*LogNode)}
			} else {
				// Prefer wildcard if exists? Drain logic is more complex, here we simplify.
				// We create exact node.
				curr.Children[token] = &LogNode{Key: token, Children: make(map[string]*LogNode)}
			}
		}
		curr = curr.Children[token]
	}

	// Leaf Logic
	if !curr.IsLeaf {
		// New Cluster
		curr.IsLeaf = true
		d.clusterCount++
		cid := fmt.Sprintf("C%d", d.clusterCount)
		curr.ClusterID = cid
		
		d.clusters[cid] = &LogCluster{
			ID: cid,
			Pattern: strings.Join(tokens, " "), // Simplified pattern
			SampleLogs: []string{logMsg},
			Count: 1,
		}
		return cid
	} 

	// Existing Cluster
	cid := curr.ClusterID
	d.clusters[cid].Count++
	// Update pattern logic could go here (merging wildcards)
	return cid
}

func (d *Drain) GetClusters() map[string]*LogCluster {
	d.mu.Lock()
	defer d.mu.Unlock()
	// copy map
	out := make(map[string]*LogCluster)
	for k, v := range d.clusters {
		c := *v
		out[k] = &c
	}
	return out
}
