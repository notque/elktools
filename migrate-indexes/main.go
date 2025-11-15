package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/olivere/elastic"
)

const (
	// DefaultBatchSize for bulk indexing operations
	DefaultBatchSize = 1000
	// DefaultScrollSize for scroll queries
	DefaultScrollSize = 1000
	// DefaultScrollTimeout for scroll context
	DefaultScrollTimeout = "5m"
	// DefaultWorkers for bulk processor
	DefaultWorkers = 2
	// DefaultFlushInterval for bulk processor
	DefaultFlushInterval = 10 * time.Second
	// ProgressInterval for logging progress
	ProgressInterval = 5000
)

var (
	// dateSuffixPattern matches date patterns like "-2024.01" or "-6-2024.01" at the end of index names
	// Supports 1-2 digit version numbers (e.g., -6-YYYY.MM, -12-YYYY.MM) or just date (e.g., -YYYY.MM)
	// Intentionally conservative: doesn't match 3+ digit numbers which are likely part of tenant IDs
	dateSuffixPattern = regexp.MustCompile(`-\d{1,2}-\d{4}\.\d{2}$|-\d{4}\.\d{2}$`)
)

// MigrationStats tracks the overall migration progress and results
type MigrationStats struct {
	TotalIndexes   int
	SuccessIndexes int
	SkippedIndexes int
	FailedIndexes  int
	TotalDocs      int
	FailedDocs     int64 // atomic counter for thread safety
}

func main() {
	// Command line flags
	var elasticHost = flag.String("elastichost", "http://localhost:9200", "OpenSearch server URL")
	var indexPattern = flag.String("pattern", "audit-*", "Index pattern to match (e.g., audit-*)")
	var targetIndex = flag.String("target", "hermes", "Target index name")
	var dryRun = flag.Bool("dry-run", false, "Dry run mode - show what would be done without making changes")
	var prefix = flag.String("prefix", "audit-", "Prefix to strip from index names to extract tenant ID")
	var username = flag.String("username", "", "OpenSearch username (optional)")
	var password = flag.String("password", "", "OpenSearch password (optional)")
	var batchSize = flag.Int("batch-size", DefaultBatchSize, "Bulk indexing batch size")
	var workers = flag.Int("workers", DefaultWorkers, "Number of bulk processor workers")
	var scrollSize = flag.Int("scroll-size", DefaultScrollSize, "Scroll batch size")
	var timeout = flag.Duration("timeout", 30*time.Minute, "Overall operation timeout")
	flag.Parse()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Connect to OpenSearch/Elasticsearch
	log.Printf("Connecting to OpenSearch at %s", *elasticHost)

	clientOptions := []elastic.ClientOptionFunc{
		elastic.SetURL(*elasticHost),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(true),
		elastic.SetHealthcheckTimeout(10 * time.Second),
	}

	// Add authentication if provided
	if *username != "" && *password != "" {
		clientOptions = append(clientOptions, elastic.SetBasicAuth(*username, *password))
		log.Printf("Using basic authentication with username: %s", *username)
	}

	client, err := elastic.NewClient(clientOptions...)
	if err != nil {
		log.Fatalf("Failed to create OpenSearch client: %v", err)
	}

	// Get cluster info
	info, code, err := client.Ping(*elasticHost).Do(ctx)
	if err != nil {
		log.Fatalf("Failed to ping OpenSearch: %v", err)
	}
	log.Printf("Connected to OpenSearch cluster %s (version %s), status code: %d", info.ClusterName, info.Version.Number, code)

	// Check if target index exists (skip in dry-run)
	if !*dryRun {
		exists, err := client.IndexExists(*targetIndex).Do(ctx)
		if err != nil {
			log.Fatalf("Failed to check target index: %v", err)
		}
		if !exists {
			log.Printf("Warning: Target index '%s' does not exist. It will be created automatically.", *targetIndex)
		}
	}

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

	// Initialize migration statistics
	stats := &MigrationStats{
		TotalIndexes: len(indexes),
	}

	// Process each index
	for _, indexName := range indexes {
		// Extract tenant ID from index name
		tenantID, err := extractTenantID(indexName, *prefix)
		if err != nil {
			log.Printf("Warning: Skipping index %s - %v", indexName, err)
			stats.SkippedIndexes++
			continue
		}

		log.Printf("\nProcessing index: %s (tenant_id: %s)", indexName, tenantID)

		// Count documents in source index
		count, err := client.Count(indexName).Do(ctx)
		if err != nil {
			log.Printf("Error counting documents in %s: %v", indexName, err)
			stats.FailedIndexes++
			continue
		}

		log.Printf("  Found %d documents to migrate", count)

		if *dryRun {
			log.Printf("  [DRY RUN] Would copy %d documents to %s with tenant_ids=[%s]", count, *targetIndex, tenantID)
			stats.TotalDocs += int(count)
			stats.SuccessIndexes++
			continue
		}

		// Copy documents from source to target
		copied, err := copyDocuments(ctx, client, indexName, *targetIndex, tenantID, *batchSize, *workers, *scrollSize, stats)
		if err != nil {
			log.Printf("Error copying documents from %s: %v", indexName, err)
			stats.FailedIndexes++
			continue
		}

		log.Printf("  Successfully copied %d documents", copied)
		stats.TotalDocs += copied
		stats.SuccessIndexes++
	}

	// Print final summary
	log.Printf("\n" + strings.Repeat("=", 70))
	log.Printf("Migration Summary:")
	log.Printf("  Total indexes found:     %d", stats.TotalIndexes)
	log.Printf("  Successfully migrated:   %d", stats.SuccessIndexes)
	log.Printf("  Skipped (invalid):       %d", stats.SkippedIndexes)
	log.Printf("  Failed:                  %d", stats.FailedIndexes)
	log.Printf("  Total documents copied:  %d", stats.TotalDocs)
	failedDocs := atomic.LoadInt64(&stats.FailedDocs)
	if failedDocs > 0 {
		log.Printf("  Failed documents:        %d", failedDocs)
	}
	if *dryRun {
		log.Printf("  Mode: DRY RUN (no changes made)")
	}
	log.Printf(strings.Repeat("=", 70))

	if stats.FailedIndexes > 0 || failedDocs > 0 {
		log.Printf("\nWarning: Migration completed with errors. Please review the logs above.")
	} else if !*dryRun {
		log.Printf("\nMigration completed successfully!")
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
func copyDocuments(ctx context.Context, client *elastic.Client, sourceIndex, targetIndex, tenantID string,
	batchSize, workers, scrollSize int, stats *MigrationStats) (int, error) {

	// Create bulk processor for efficient indexing with error handling
	bulkProcessor, err := client.BulkProcessor().
		Name("index-migrator").
		Workers(workers).
		BulkActions(batchSize).
		FlushInterval(DefaultFlushInterval).
		After(func(executionId int64, requests []elastic.BulkableRequest, response *elastic.BulkResponse, err error) {
			if err != nil {
				log.Printf("  Bulk request %d failed: %v", executionId, err)
				atomic.AddInt64(&stats.FailedDocs, int64(len(requests)))
				return
			}
			if response != nil && response.Errors {
				// Count individual document failures
				failCount := 0
				for _, item := range response.Items {
					for action, result := range item {
						if result.Error != nil {
							log.Printf("  Failed to %s document %s: %v", action, result.Id, result.Error)
							failCount++
						}
					}
				}
				if failCount > 0 {
					atomic.AddInt64(&stats.FailedDocs, int64(failCount))
				}
			}
		}).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create bulk processor: %w", err)
	}
	defer bulkProcessor.Close()

	// Use scroll to iterate through all documents
	scroll := client.Scroll(sourceIndex).
		Size(scrollSize).
		KeepAlive(DefaultScrollTimeout)

	var scrollId string
	defer func() {
		// Clean up scroll context
		if scrollId != "" {
			_, err := client.ClearScroll(scrollId).Do(context.Background())
			if err != nil {
				log.Printf("  Warning: Failed to clear scroll context: %v", err)
			}
		}
	}()

	docCount := 0
	lastProgress := 0

	for {
		results, err := scroll.Do(ctx)
		if err != nil {
			return docCount, fmt.Errorf("scroll error: %w", err)
		}

		if results == nil || results.Hits == nil || len(results.Hits.Hits) == 0 {
			// No more documents
			break
		}

		// Store scroll ID for cleanup
		scrollId = results.ScrollId

		// Process each document in the current batch
		for _, hit := range results.Hits.Hits {
			// Parse the document
			var doc map[string]interface{}
			if err := json.Unmarshal(*hit.Source, &doc); err != nil {
				log.Printf("  Warning: Failed to unmarshal document %s: %v", hit.Id, err)
				atomic.AddInt64(&stats.FailedDocs, 1)
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

			// Log progress periodically
			if docCount-lastProgress >= ProgressInterval {
				log.Printf("  Progress: %d documents processed...", docCount)
				lastProgress = docCount
			}
		}

		// Check if we should continue scrolling
		if len(results.Hits.Hits) < scrollSize {
			break
		}
	}

	// Wait for all bulk requests to complete
	if err := bulkProcessor.Flush(); err != nil {
		return docCount, fmt.Errorf("failed to flush bulk processor: %w", err)
	}

	return docCount, nil
}
