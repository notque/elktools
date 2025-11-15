# OpenSearch Index Migration Tool

This tool migrates documents from multiple tenant-specific audit indexes to a single unified index using tenant filtering.

## Overview

The migration tool:
1. Connects to OpenSearch/Elasticsearch
2. Finds all indexes matching a pattern (default: `audit-*`)
3. Extracts the tenant ID from each index name
4. Copies all documents from each source index to the target index
5. Adds a `tenant_ids` field (keyword array) to each document for filtering

## Usage

### Basic Usage

```bash
go run main.go -elastichost http://localhost:9200
```

### Command Line Options

- `-elastichost` - OpenSearch server URL (default: `http://localhost:9200`)
- `-pattern` - Index pattern to match (default: `audit-*`)
- `-target` - Target index name (default: `hermes`)
- `-prefix` - Prefix to strip from index names to extract tenant ID (default: `audit-`)
- `-dry-run` - Dry run mode - shows what would be done without making changes

### Examples

#### Dry Run (Preview)
```bash
go run main.go \
  -elastichost http://opensearch.example.com:9200 \
  -dry-run
```

#### Migrate with Custom Target Index
```bash
go run main.go \
  -elastichost http://opensearch.example.com:9200 \
  -target my-unified-index
```

#### Migrate Specific Pattern
```bash
go run main.go \
  -elastichost http://opensearch.example.com:9200 \
  -pattern "audit-production-*"
```

## Migration Process

The tool extracts the OpenStack project ID from index names by removing the prefix and any date suffix.

**Examples:**
- `audit-tenant123-2024.01` → tenant ID: `tenant123`
- `audit-abc-def-123-456-789-2024.01` → tenant ID: `abc-def-123-456-789`
- `audit-project456-6-2024.11` → tenant ID: `project456`

**Migration Steps:**
1. Extracts tenant ID (OpenStack project ID) from index name
2. Reads all documents using scroll API
3. Adds `tenant_ids: ["<project_id>"]` to each document
4. Bulk indexes to target index (default: `hermes`)

## Document Structure

### Before Migration
```json
{
  "@timestamp": "2024-01-15T10:30:00Z",
  "action": "create",
  "outcome": "success"
}
```

### After Migration
```json
{
  "tenant_ids": ["tenant123"],
  "@timestamp": "2024-01-15T10:30:00Z",
  "action": "create",
  "outcome": "success"
}
```

**Note:** The `tenant_ids` field contains the OpenStack project ID without the date suffix from the index name.

## Performance

- Uses bulk indexing with batch size of 1000 documents
- Scroll query size: 1000 documents per batch
- 2 parallel workers for bulk processing
- Auto-flushes every 10 seconds

## Testing

This project includes comprehensive unit and integration tests that run against a real OpenSearch instance in Docker.

### Prerequisites

- Docker and Docker Compose
- Go 1.13 or later

### Quick Start

The easiest way to run all tests:

```bash
# Using the test script
./run-tests.sh

# Or using Make
make test
```

### Test Options

#### Run All Tests
```bash
make test                  # Starts Docker, runs unit + integration tests
```

#### Run Unit Tests Only
```bash
make test-unit            # No Docker required
```

#### Run Integration Tests Only
```bash
make docker-up            # Start OpenSearch
make test-integration     # Run tests against OpenSearch
make docker-down          # Stop and cleanup
```

#### Manual Docker Control
```bash
# Start OpenSearch
docker-compose up -d

# Wait for it to be ready
curl http://localhost:9200/_cluster/health

# Run tests
go test -v -count=1

# Stop and cleanup
docker-compose down -v
```

### Test Coverage

**Unit Tests:**
- `TestExtractTenantID` - Tests tenant ID extraction from index names

**Integration Tests:**
- `TestGetMatchingIndexes` - Verifies index pattern matching
- `TestCopyDocuments` - Tests document migration with tenant_ids field
- `TestEndToEndMigration` - Complete migration workflow with multiple indexes

### OpenSearch Test Instance

The test environment uses:
- **Image:** `opensearchproject/opensearch:2.11.0`
- **Port:** 9200
- **Mode:** Single-node
- **Security:** Disabled for easier testing
- **Memory:** 512MB heap

### Skipping Integration Tests

To skip integration tests (e.g., in CI without Docker):

```bash
SKIP_INTEGRATION=1 go test -v
```

### Viewing OpenSearch Logs

```bash
make docker-logs
# or
docker-compose logs -f opensearch
```

### Cleanup

```bash
make clean    # Removes binary, stops Docker, clears test cache
```

## Notes

- The tool preserves document IDs from source indexes
- If a document ID already exists in the target index, it will be overwritten
- The `tenant_ids` field is created as a keyword array to support multiple tenants per document in the future
- Currently, each document gets a single tenant ID based on its source index
