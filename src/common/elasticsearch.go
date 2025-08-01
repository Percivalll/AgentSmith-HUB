package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

// ElasticsearchAuthConfig represents authentication configuration for Elasticsearch
type ElasticsearchAuthConfig struct {
	Type     string `yaml:"type"`               // auth type: basic, api_key, bearer
	Username string `yaml:"username,omitempty"` // for basic auth
	Password string `yaml:"password,omitempty"` // for basic auth
	APIKey   string `yaml:"api_key,omitempty"`  // for api_key auth
	Token    string `yaml:"token,omitempty"`    // for bearer token auth
}

// ElasticsearchProducer wraps the Elasticsearch client with a channel-based interface
type ElasticsearchProducer struct {
	Client        *elasticsearch.Client
	MsgChan       chan map[string]interface{}
	Index         string
	IndexTemplate string // Store the original index template for time pattern replacement
	batchSize     int
	flushDur      time.Duration
	maxRetries    int
	retryDelay    time.Duration
	stopChan      chan struct{} // Add stop channel for graceful shutdown
}

// replaceTimePatterns replaces time patterns in index name with actual values
func replaceTimePatterns(indexTemplate string) string {
	now := time.Now()

	// Replace various time patterns
	replacements := map[string]string{
		"{YYYY}":       now.Format("2006"),
		"{YY}":         now.Format("06"),
		"{MM}":         now.Format("01"),
		"{DD}":         now.Format("02"),
		"{HH}":         now.Format("15"),
		"{mm}":         now.Format("04"),
		"{ss}":         now.Format("05"),
		"{YYYY.MM.DD}": now.Format("2006.01.02"),
		"{YYYY-MM-DD}": now.Format("2006-01-02"),
		"{YYYY/MM/DD}": now.Format("2006/01/02"),
		"{YYYY_MM_DD}": now.Format("2006_01_02"),
		"{YYYY.MM}":    now.Format("2006.01"),
		"{YYYY-MM}":    now.Format("2006-01"),
		"{YYYY/MM}":    now.Format("2006/01"),
		"{YYYY_MM}":    now.Format("2006_01"),
	}

	result := indexTemplate
	for pattern, replacement := range replacements {
		result = strings.ReplaceAll(result, pattern, replacement)
	}

	return result
}

// NewElasticsearchProducer creates a new Elasticsearch producer
func NewElasticsearchProducer(hosts []string, index string, msgChan chan map[string]interface{}, batchSize int, flushDur time.Duration, auth *ElasticsearchAuthConfig) (*ElasticsearchProducer, error) {
	cfg := elasticsearch.Config{
		Addresses:     hosts,
		MaxRetries:    3,
		RetryOnStatus: []int{502, 503, 504, 429},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip TLS certificate verification
			},
		},
	}

	// Configure authentication if provided
	if auth != nil {
		switch auth.Type {
		case "basic":
			if auth.Username != "" && auth.Password != "" {
				cfg.Username = auth.Username
				cfg.Password = auth.Password
			}
		case "api_key":
			if auth.APIKey != "" {
				cfg.APIKey = auth.APIKey
			}
		case "bearer":
			if auth.Token != "" {
				cfg.Header = http.Header{
					"Authorization": []string{"Bearer " + auth.Token},
				}
			}
		}
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ES client: %v", err)
	}

	// Replace time patterns in index name
	resolvedIndex := replaceTimePatterns(index)

	prod := &ElasticsearchProducer{
		Client:        client,
		MsgChan:       msgChan,
		Index:         resolvedIndex,
		IndexTemplate: index, // Store original template for potential future use
		batchSize:     batchSize,
		flushDur:      flushDur,
		maxRetries:    3,
		retryDelay:    1 * time.Second,
		stopChan:      make(chan struct{}),
	}

	go prod.run()
	return prod, nil
}

func (p *ElasticsearchProducer) run() {
	batch := make([]map[string]interface{}, 0, p.batchSize)
	timer := time.NewTimer(p.flushDur)
	defer timer.Stop()

	for {
		select {
		case <-p.stopChan:
			// Stop timer to prevent any further timer events
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			// Don't flush remaining batch during shutdown to avoid blocking
			// Just return immediately to ensure fast shutdown
			return
		case msg, ok := <-p.MsgChan:
			if !ok {
				// Channel is closed, flush any remaining batch
				if len(batch) > 0 {
					p.flush(batch)
				}
				return
			}
			batch = append(batch, msg)
			if len(batch) >= p.batchSize {
				p.flush(batch)
				batch = batch[:0]
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(p.flushDur)
			}
		case <-timer.C:
			if len(batch) > 0 {
				p.flush(batch)
				batch = batch[:0]
			}
			timer.Reset(p.flushDur)
		}
	}
}

// sendBatch sends a batch of documents to Elasticsearch with retry logic
func (p *ElasticsearchProducer) sendBatch(batch []map[string]interface{}) {
	if len(batch) == 0 {
		return
	}

	var buf bytes.Buffer
	for _, doc := range batch {
		// Add index action
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": p.Index,
			},
		}
		if err := json.NewEncoder(&buf).Encode(meta); err != nil {
			fmt.Printf("Failed to encode meta: %v\n", err)
			continue
		}
		// Add document
		if err := json.NewEncoder(&buf).Encode(doc); err != nil {
			fmt.Printf("Failed to encode document: %v\n", err)
			continue
		}
	}

	// Try to send with retries and timeout control
	for i := 0; i <= p.maxRetries; i++ {
		// Create context with timeout for each retry
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Use context for bulk request
		res, err := p.Client.Bulk(bytes.NewReader(buf.Bytes()), p.Client.Bulk.WithContext(ctx))

		cancel() // Always cancel context

		if err != nil {
			if i == p.maxRetries {
				fmt.Printf("Failed to send batch to ES after %d retries: %v\n", p.maxRetries, err)
				return
			}
			time.Sleep(p.retryDelay)
			continue
		}
		defer res.Body.Close()

		if res.IsError() {
			if i == p.maxRetries {
				fmt.Printf("ES returned error after %d retries: %s\n", p.maxRetries, res.String())
				return
			}
			time.Sleep(p.retryDelay)
			continue
		}

		// Success
		return
	}
}

// flush batch writes to ES
func (p *ElasticsearchProducer) flush(batch []map[string]interface{}) {
	p.sendBatch(batch)
}

// Close closes the producer
// Note: We don't close MsgChan here because it's owned by the caller
func (p *ElasticsearchProducer) Close() {
	// Signal the goroutine to stop
	if p.stopChan != nil {
		close(p.stopChan)
	}
	// The channel will be closed by the owner (output component)
	// We just need to ensure any pending operations are completed
}

// TestConnection tests the connection to Elasticsearch cluster
// This method creates a temporary client to test connectivity without affecting the main producer
func TestElasticsearchConnection(hosts []string, auth *ElasticsearchAuthConfig) error {
	cfg := elasticsearch.Config{
		Addresses:     hosts,
		MaxRetries:    1,
		RetryOnStatus: []int{502, 503, 504, 429},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip TLS certificate verification
			},
		},
	}

	// Configure authentication if provided
	if auth != nil {
		switch auth.Type {
		case "basic":
			if auth.Username != "" && auth.Password != "" {
				cfg.Username = auth.Username
				cfg.Password = auth.Password
			}
		case "api_key":
			if auth.APIKey != "" {
				cfg.APIKey = auth.APIKey
			}
		case "bearer":
			if auth.Token != "" {
				cfg.Header = http.Header{
					"Authorization": []string{"Bearer " + auth.Token},
				}
			}
		}
	}

	// Create a temporary client for testing
	testClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create test client: %w", err)
	}

	// Test connection by pinging the cluster
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := testClient.Ping(testClient.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to ping Elasticsearch cluster: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("Elasticsearch cluster returned error: %s", res.String())
	}

	return nil
}

// TestIndexExists tests if a specific index exists in Elasticsearch
func TestElasticsearchIndexExists(hosts []string, index string, auth *ElasticsearchAuthConfig) (bool, error) {
	cfg := elasticsearch.Config{
		Addresses:     hosts,
		MaxRetries:    1,
		RetryOnStatus: []int{502, 503, 504, 429},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip TLS certificate verification
			},
		},
	}

	// Configure authentication if provided
	if auth != nil {
		switch auth.Type {
		case "basic":
			if auth.Username != "" && auth.Password != "" {
				cfg.Username = auth.Username
				cfg.Password = auth.Password
			}
		case "api_key":
			if auth.APIKey != "" {
				cfg.APIKey = auth.APIKey
			}
		case "bearer":
			if auth.Token != "" {
				cfg.Header = http.Header{
					"Authorization": []string{"Bearer " + auth.Token},
				}
			}
		}
	}

	// Create a temporary client for testing
	testClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return false, fmt.Errorf("failed to create test client: %w", err)
	}

	// Test index existence
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := testClient.Indices.Exists([]string{index}, testClient.Indices.Exists.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to check index existence: %w", err)
	}
	defer res.Body.Close()

	// 200 means index exists, 404 means index doesn't exist
	if res.StatusCode == 200 {
		return true, nil
	} else if res.StatusCode == 404 {
		return false, nil
	} else {
		return false, fmt.Errorf("unexpected response when checking index: %s", res.String())
	}
}

// GetElasticsearchClusterInfo gets basic cluster information
func GetElasticsearchClusterInfo(hosts []string, auth *ElasticsearchAuthConfig) (map[string]interface{}, error) {
	cfg := elasticsearch.Config{
		Addresses:     hosts,
		MaxRetries:    1,
		RetryOnStatus: []int{502, 503, 504, 429},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip TLS certificate verification
			},
		},
	}

	// Configure authentication if provided
	if auth != nil {
		switch auth.Type {
		case "basic":
			if auth.Username != "" && auth.Password != "" {
				cfg.Username = auth.Username
				cfg.Password = auth.Password
			}
		case "api_key":
			if auth.APIKey != "" {
				cfg.APIKey = auth.APIKey
			}
		case "bearer":
			if auth.Token != "" {
				cfg.Header = http.Header{
					"Authorization": []string{"Bearer " + auth.Token},
				}
			}
		}
	}

	// Create a temporary client for testing
	testClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create test client: %w", err)
	}

	// Get cluster info
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := testClient.Info(testClient.Info.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("Elasticsearch cluster returned error: %s", res.String())
	}

	var clusterInfo map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&clusterInfo); err != nil {
		return nil, fmt.Errorf("failed to decode cluster info: %w", err)
	}

	return clusterInfo, nil
}
