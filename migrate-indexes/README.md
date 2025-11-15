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

For an index named `audit-tenant123-2024.01`:
1. Extracts tenant ID: `tenant123-2024.01`
2. Reads all documents using scroll API
3. Adds `tenant_ids: ["tenant123-2024.01"]` to each document
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
  "tenant_ids": ["tenant123-2024.01"],
  "@timestamp": "2024-01-15T10:30:00Z",
  "action": "create",
  "outcome": "success"
}
```

## Performance

- Uses bulk indexing with batch size of 1000 documents
- Scroll query size: 1000 documents per batch
- 2 parallel workers for bulk processing
- Auto-flushes every 10 seconds

## Notes

- The tool preserves document IDs from source indexes
- If a document ID already exists in the target index, it will be overwritten
- The `tenant_ids` field is created as a keyword array to support multiple tenants per document in the future
- Currently, each document gets a single tenant ID based on its source index
