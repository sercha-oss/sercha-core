package opensearch

import (
	"crypto/tls"
	"net/http"
	"time"

	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// SearchEngine implements the driven.SearchEngine port using OpenSearch
type SearchEngine struct {
	client    *opensearchapi.Client
	indexName string
	timeout   time.Duration
}

// Config holds OpenSearch connection configuration
type Config struct {
	// URL is the OpenSearch endpoint (e.g., "http://localhost:9200")
	URL string

	// IndexName is the name of the index to use for chunks
	IndexName string

	// Timeout for requests
	Timeout time.Duration

	// InsecureSkipVerify disables TLS certificate verification (development only)
	InsecureSkipVerify bool
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		URL:                "http://localhost:9200",
		IndexName:          "sercha_chunks",
		Timeout:            30 * time.Second,
		InsecureSkipVerify: false,
	}
}

// NewSearchEngine creates a new OpenSearch search engine adapter
func NewSearchEngine(cfg Config) (*SearchEngine, error) {
	// Build OpenSearch client configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{cfg.URL},
			Transport: transport,
		},
	})
	if err != nil {
		return nil, err
	}

	return &SearchEngine{
		client:    client,
		indexName: cfg.IndexName,
		timeout:   cfg.Timeout,
	}, nil
}
