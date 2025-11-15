package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/olivere/elastic"
)

const (
	// BatchSize for bulk indexing operations
	BatchSize = 1000
	// ScrollSize for scroll queries
	ScrollSize = 1000
	// ScrollTimeout for scroll context
	ScrollTimeout = "5m"
)

var (
	// dateSuffixPattern matches date patterns like "-2024.01" or "-6-2024.01" at the end of index names
	// Single digit version (e.g., -6-YYYY.MM) or just date (e.g., -YYYY.MM)
	dateSuffixPattern = regexp.MustCompile(`-\d-\d{4}\.\d{2}$|-\d{4}\.\d{2}$`)
)

func main() {
	// Command line flags
	var elasticHost = flag.String("elastichost", "http://localhost:9200", "OpenSearch server URL")
	var indexPattern = flag.String("pattern", "audit-*", "Index pattern to match (e.g., audit-*)")
	var targetIndex = flag.String("target", "hermes", "Target index name")
	var dryRun = flag.Bool("dry-run", false, "Dry run mode - show what would be done without making changes")
	var prefix = flag.String("prefix", "audit-", "Prefix to strip from index names to extract tenant ID")
	flag.Parse()

	// Connect to OpenSearch/Elasticsearch
	log.Printf("Connecting to OpenSearch at %s", *elasticHost)
	client, err := elastic.NewClient(
		elastic.SetURL(*elasticHost),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(true),
		elastic.SetHealthcheckTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenSearch client: %v", err)
	}

	ctx := context.Background()

	// Get cluster info
	info, code, err := client.Ping(*elasticHost).Do(ctx)
	if err != nil {
		log.Fatalf("Failed to ping OpenSearch: %v", err)
	}
	log.Printf("Connected to OpenSearch cluster %s (version %s), status code: %d", info.ClusterName, info.Version.Number, code)

	// Get all indexes matching the pattern
	indexes, err := getMatchingIndexes(ctx, client, *indexPattern)
	if err != nil {
		log.Fatalf("Failed to get matching indexes: %v", err)
	}

	if len(indexes) == 0 {
		log.Printf("No indexes found matching pattern: %s", *indexPattern)
		return
	}

	log.Printf("Found %d indexes matching pattern '%s'", len(indexes), *indexPattern)

	// Process each index
	totalDocs := 0
	for _, indexName := range indexes {
		// Extract tenant ID from index name
		tenantID, err := extractTenantID(indexName, *prefix)
		if err != nil {
			log.Printf("Warning: Skipping index %s - %v", indexName, err)
			continue
		}

		log.Printf("\nProcessing index: %s (tenant_id: %s)", indexName, tenantID)

		// Count documents in source index
		count, err := client.Count(indexName).Do(ctx)
		if err != nil {
			log.Printf("Error counting documents in %s: %v", indexName, err)
			continue
		}

		log.Printf("  Found %d documents to migrate", count)

		if *dryRun {
			log.Printf("  [DRY RUN] Would copy %d documents to %s with tenant_ids=[%s]", count, *targetIndex, tenantID)
			totalDocs += int(count)
			continue
		}

		// Copy documents from source to target
		copied, err := copyDocuments(ctx, client, indexName, *targetIndex, tenantID)
		if err != nil {
			log.Printf("Error copying documents from %s: %v", indexName, err)
			continue
		}

		log.Printf("  Successfully copied %d documents", copied)
		totalDocs += copied
	}

	if *dryRun {
		log.Printf("\n[DRY RUN] Would migrate %d total documents from %d indexes to %s", totalDocs, len(indexes), *targetIndex)
	} else {
		log.Printf("\nMigration complete! Migrated %d total documents from %d indexes to %s", totalDocs, len(indexes), *targetIndex)
	}
}

// getMatchingIndexes returns all index names matching the given pattern
func getMatchingIndexes(ctx context.Context, client *elastic.Client, pattern string) ([]string, error) {
	catIndexService := client.CatIndices().Index(pattern)
	indices, err := catIndexService.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	var indexNames []string
	for _, index := range indices {
		indexNames = append(indexNames, index.Index)
	}

	return indexNames, nil
}

// extractTenantID extracts the tenant ID (OpenStack project ID) from an index name
// by removing the prefix and any date suffix.
// For example: "audit-abc123-2024.01" -> "abc123"
// For example: "audit-abc123-6-2024.01" -> "abc123"
func extractTenantID(indexName, prefix string) (string, error) {
	if !strings.HasPrefix(indexName, prefix) {
		return "", fmt.Errorf("index %s does not have prefix %s", indexName, prefix)
	}

	tenantID := strings.TrimPrefix(indexName, prefix)
	if tenantID == "" {
		return "", fmt.Errorf("tenant ID is empty after removing prefix")
	}

	// Remove date suffix patterns like "-2024.01" or "-6-2024.01"
	tenantID = dateSuffixPattern.ReplaceAllString(tenantID, "")

	if tenantID == "" {
		return "", fmt.Errorf("tenant ID is empty after removing date suffix")
	}

	return tenantID, nil
}

// copyDocuments copies all documents from source index to target index,
// adding the tenant_ids field to each document
func copyDocuments(ctx context.Context, client *elastic.Client, sourceIndex, targetIndex, tenantID string) (int, error) {
	// Create bulk processor for efficient indexing
	bulkProcessor, err := client.BulkProcessor().
		Name("index-migrator").
		Workers(2).
		BulkActions(BatchSize).
		FlushInterval(10 * time.Second).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create bulk processor: %w", err)
	}
	defer bulkProcessor.Close()

	// Use scroll to iterate through all documents
	scroll := client.Scroll(sourceIndex).
		Size(ScrollSize).
		KeepAlive(ScrollTimeout)

	docCount := 0

	for {
		results, err := scroll.Do(ctx)
		if err != nil {
			return docCount, fmt.Errorf("scroll error: %w", err)
		}

		if results == nil || results.Hits == nil || len(results.Hits.Hits) == 0 {
			// No more documents
			break
		}

		// Process each document in the current batch
		for _, hit := range results.Hits.Hits {
			// Parse the document
			var doc map[string]interface{}
			if err := json.Unmarshal(*hit.Source, &doc); err != nil {
				log.Printf("Warning: Failed to unmarshal document %s: %v", hit.Id, err)
				continue
			}

			// Add tenant_ids field at the top level
			// Using array format as mentioned (keyword field with comma-delimited values)
			// For now, each document gets a single-element array with the tenant ID
			doc["tenant_ids"] = []string{tenantID}

			// Create bulk index request
			req := elastic.NewBulkIndexRequest().
				Index(targetIndex).
				Type("_doc").
				Id(hit.Id).
				Doc(doc)

			bulkProcessor.Add(req)
			docCount++
		}

		// Check if we should continue scrolling
		if len(results.Hits.Hits) < ScrollSize {
			break
		}
	}

	// Wait for all bulk requests to complete
	if err := bulkProcessor.Flush(); err != nil {
		return docCount, fmt.Errorf("failed to flush bulk processor: %w", err)
	}

	return docCount, nil
}
