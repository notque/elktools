package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/olivere/elastic"
)

const (
	testElasticHost = "http://localhost:9200"
	testTimeout     = 30 * time.Second
)

// Unit Tests

func TestExtractTenantID(t *testing.T) {
	tests := []struct {
		name      string
		indexName string
		prefix    string
		wantID    string
		wantError bool
	}{
		{
			name:      "basic tenant extraction",
			indexName: "audit-tenant123",
			prefix:    "audit-",
			wantID:    "tenant123",
			wantError: false,
		},
		{
			name:      "tenant with date suffix",
			indexName: "audit-tenant456-2024.01",
			prefix:    "audit-",
			wantID:    "tenant456",
			wantError: false,
		},
		{
			name:      "tenant with version and date suffix",
			indexName: "audit-tenant789-6-2024.11",
			prefix:    "audit-",
			wantID:    "tenant789",
			wantError: false,
		},
		{
			name:      "complex tenant ID with date",
			indexName: "audit-prod-tenant-abc-123-2024.11",
			prefix:    "audit-",
			wantID:    "prod-tenant-abc-123",
			wantError: false,
		},
		{
			name:      "UUID-like tenant ID with date",
			indexName: "audit-abc-def-123-456-789-2024.01",
			prefix:    "audit-",
			wantID:    "abc-def-123-456-789",
			wantError: false,
		},
		{
			name:      "wrong prefix",
			indexName: "logs-tenant123",
			prefix:    "audit-",
			wantID:    "",
			wantError: true,
		},
		{
			name:      "empty after prefix",
			indexName: "audit-",
			prefix:    "audit-",
			wantID:    "",
			wantError: true,
		},
		{
			name:      "only date after prefix (no leading dash)",
			indexName: "audit-2024.01",
			prefix:    "audit-",
			wantID:    "2024.01",
			wantError: false,
		},
		{
			name:      "only version and date after prefix",
			indexName: "audit-6-2024.01",
			prefix:    "audit-",
			wantID:    "6",
			wantError: false,
		},
		{
			name:      "custom prefix without date",
			indexName: "logs-production-tenant789",
			prefix:    "logs-production-",
			wantID:    "tenant789",
			wantError: false,
		},
		{
			name:      "custom prefix with date",
			indexName: "logs-production-tenant789-2025.03",
			prefix:    "logs-production-",
			wantID:    "tenant789",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, err := extractTenantID(tt.indexName, tt.prefix)

			if tt.wantError {
				if err == nil {
					t.Errorf("extractTenantID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("extractTenantID() unexpected error: %v", err)
				return
			}

			if gotID != tt.wantID {
				t.Errorf("extractTenantID() = %v, want %v", gotID, tt.wantID)
			}
		})
	}
}

// Integration Tests

func TestIntegration(t *testing.T) {
	// Skip integration tests if SKIP_INTEGRATION is set
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration tests (SKIP_INTEGRATION is set)")
	}

	// Connect to test OpenSearch instance
	ctx := context.Background()
	client, err := connectToTestOpenSearch(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to test OpenSearch: %v", err)
	}

	// Clean up before and after tests
	defer cleanupTestIndexes(ctx, client, t)
	cleanupTestIndexes(ctx, client, t)

	t.Run("GetMatchingIndexes", testGetMatchingIndexes(ctx, client))
	t.Run("CopyDocuments", testCopyDocuments(ctx, client))
	t.Run("EndToEndMigration", testEndToEndMigration(ctx, client))
}

func connectToTestOpenSearch(ctx context.Context) (*elastic.Client, error) {
	client, err := elastic.NewClient(
		elastic.SetURL(testElasticHost),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(true),
		elastic.SetHealthcheckTimeout(testTimeout),
	)
	if err != nil {
		return nil, err
	}

	// Wait for cluster to be ready
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		_, _, err := client.Ping(testElasticHost).Do(ctx)
		if err == nil {
			return client, nil
		}
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for OpenSearch to be ready")
}

func cleanupTestIndexes(ctx context.Context, client *elastic.Client, t *testing.T) {
	indexes := []string{
		"audit-test-tenant1",
		"audit-test-tenant2",
		"audit-test-tenant3",
		"test-hermes",
	}

	for _, idx := range indexes {
		_, _ = client.DeleteIndex(idx).Do(ctx)
	}
}

func testGetMatchingIndexes(ctx context.Context, client *elastic.Client) func(*testing.T) {
	return func(t *testing.T) {
		// Create test indexes
		testIndexes := []string{
			"audit-test-tenant1",
			"audit-test-tenant2",
			"audit-test-tenant3",
		}

		for _, idx := range testIndexes {
			_, err := client.CreateIndex(idx).Do(ctx)
			if err != nil {
				t.Fatalf("Failed to create test index %s: %v", idx, err)
			}
		}

		// Wait for indexes to be created
		time.Sleep(1 * time.Second)

		// Test getting matching indexes
		indexes, err := getMatchingIndexes(ctx, client, "audit-test-*")
		if err != nil {
			t.Fatalf("getMatchingIndexes() error: %v", err)
		}

		if len(indexes) != 3 {
			t.Errorf("getMatchingIndexes() returned %d indexes, want 3", len(indexes))
		}

		// Verify all expected indexes are present
		indexMap := make(map[string]bool)
		for _, idx := range indexes {
			indexMap[idx] = true
		}

		for _, expected := range testIndexes {
			if !indexMap[expected] {
				t.Errorf("Expected index %s not found in results", expected)
			}
		}
	}
}

func testCopyDocuments(ctx context.Context, client *elastic.Client) func(*testing.T) {
	return func(t *testing.T) {
		sourceIndex := "audit-test-tenant1"
		targetIndex := "test-hermes"
		tenantID := "test-tenant1"

		// Create source index
		_, err := client.CreateIndex(sourceIndex).Do(ctx)
		if err != nil {
			t.Fatalf("Failed to create source index: %v", err)
		}

		// Create test documents
		testDocs := []map[string]interface{}{
			{
				"@timestamp": "2024-01-15T10:00:00Z",
				"action":     "create",
				"outcome":    "success",
				"user_id":    "user1",
			},
			{
				"@timestamp": "2024-01-15T11:00:00Z",
				"action":     "delete",
				"outcome":    "success",
				"user_id":    "user2",
			},
			{
				"@timestamp": "2024-01-15T12:00:00Z",
				"action":     "update",
				"outcome":    "failure",
				"user_id":    "user3",
			},
		}

		// Index test documents
		for i, doc := range testDocs {
			_, err := client.Index().
				Index(sourceIndex).
				Type("_doc").
				Id(fmt.Sprintf("doc-%d", i)).
				BodyJson(doc).
				Refresh("true").
				Do(ctx)
			if err != nil {
				t.Fatalf("Failed to index test document: %v", err)
			}
		}

		// Wait for documents to be indexed
		time.Sleep(1 * time.Second)

		// Copy documents
		copied, err := copyDocuments(ctx, client, sourceIndex, targetIndex, tenantID)
		if err != nil {
			t.Fatalf("copyDocuments() error: %v", err)
		}

		if copied != len(testDocs) {
			t.Errorf("copyDocuments() copied %d documents, want %d", copied, len(testDocs))
		}

		// Refresh target index
		_, err = client.Refresh(targetIndex).Do(ctx)
		if err != nil {
			t.Fatalf("Failed to refresh target index: %v", err)
		}

		// Verify documents in target index
		time.Sleep(1 * time.Second)
		count, err := client.Count(targetIndex).Do(ctx)
		if err != nil {
			t.Fatalf("Failed to count documents in target index: %v", err)
		}

		if count != int64(len(testDocs)) {
			t.Errorf("Target index has %d documents, want %d", count, len(testDocs))
		}

		// Verify tenant_ids field was added
		searchResult, err := client.Search(targetIndex).
			Size(10).
			Do(ctx)
		if err != nil {
			t.Fatalf("Failed to search target index: %v", err)
		}

		for _, hit := range searchResult.Hits.Hits {
			var doc map[string]interface{}
			if err := json.Unmarshal(*hit.Source, &doc); err != nil {
				t.Fatalf("Failed to unmarshal document: %v", err)
			}

			// Check tenant_ids field exists
			tenantIDs, ok := doc["tenant_ids"]
			if !ok {
				t.Error("Document missing tenant_ids field")
				continue
			}

			// Verify tenant_ids is an array
			tenantIDsArray, ok := tenantIDs.([]interface{})
			if !ok {
				t.Errorf("tenant_ids is not an array: %T", tenantIDs)
				continue
			}

			// Verify array contains the correct tenant ID
			if len(tenantIDsArray) != 1 {
				t.Errorf("tenant_ids has %d elements, want 1", len(tenantIDsArray))
				continue
			}

			if tenantIDsArray[0] != tenantID {
				t.Errorf("tenant_ids[0] = %v, want %v", tenantIDsArray[0], tenantID)
			}

			// Verify original fields are preserved
			if _, ok := doc["action"]; !ok {
				t.Error("Original field 'action' not preserved")
			}
			if _, ok := doc["outcome"]; !ok {
				t.Error("Original field 'outcome' not preserved")
			}
		}
	}
}

func testEndToEndMigration(ctx context.Context, client *elastic.Client) func(*testing.T) {
	return func(t *testing.T) {
		// Create multiple source indexes with documents
		sourceIndexes := map[string]string{
			"audit-tenant-alpha": "tenant-alpha",
			"audit-tenant-beta":  "tenant-beta",
			"audit-tenant-gamma": "tenant-gamma",
		}

		targetIndex := "test-hermes"
		totalDocs := 0

		// Create and populate source indexes
		for indexName, tenantID := range sourceIndexes {
			_, err := client.CreateIndex(indexName).Do(ctx)
			if err != nil {
				t.Fatalf("Failed to create index %s: %v", indexName, err)
			}

			// Add 5 documents per index
			for i := 0; i < 5; i++ {
				doc := map[string]interface{}{
					"@timestamp": time.Now().Format(time.RFC3339),
					"action":     fmt.Sprintf("action-%d", i),
					"outcome":    "success",
					"tenant":     tenantID,
				}

				_, err := client.Index().
					Index(indexName).
					Type("_doc").
					Id(fmt.Sprintf("%s-doc-%d", tenantID, i)).
					BodyJson(doc).
					Refresh("true").
					Do(ctx)
				if err != nil {
					t.Fatalf("Failed to index document: %v", err)
				}
				totalDocs++
			}
		}

		// Wait for all documents to be indexed
		time.Sleep(2 * time.Second)

		// Migrate all indexes
		migratedDocs := 0
		for indexName, tenantID := range sourceIndexes {
			copied, err := copyDocuments(ctx, client, indexName, targetIndex, tenantID)
			if err != nil {
				t.Fatalf("Failed to copy documents from %s: %v", indexName, err)
			}
			migratedDocs += copied
		}

		if migratedDocs != totalDocs {
			t.Errorf("Migrated %d documents, want %d", migratedDocs, totalDocs)
		}

		// Refresh target index
		_, err := client.Refresh(targetIndex).Do(ctx)
		if err != nil {
			t.Fatalf("Failed to refresh target index: %v", err)
		}

		// Verify total count in target index
		time.Sleep(1 * time.Second)
		count, err := client.Count(targetIndex).Do(ctx)
		if err != nil {
			t.Fatalf("Failed to count documents: %v", err)
		}

		if count != int64(totalDocs) {
			t.Errorf("Target index has %d documents, want %d", count, totalDocs)
		}

		// Verify we can query by tenant_ids
		for _, tenantID := range sourceIndexes {
			termQuery := elastic.NewTermQuery("tenant_ids", tenantID)
			searchResult, err := client.Search(targetIndex).
				Query(termQuery).
				Do(ctx)
			if err != nil {
				t.Fatalf("Failed to search by tenant_ids: %v", err)
			}

			if searchResult.Hits.TotalHits != 5 {
				t.Errorf("Search for tenant_ids=%s returned %d hits, want 5", tenantID, searchResult.Hits.TotalHits)
			}
		}

		t.Logf("Successfully migrated %d documents from %d indexes", migratedDocs, len(sourceIndexes))
	}
}
