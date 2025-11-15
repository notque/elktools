# Pull Request: Add OpenSearch index migration tool for tenant consolidation

**Branch:** `claude/opensearch-index-copy-01RUfwRWoLTRr5MJ2QqoqB5T`
**Base:** `master`

## Summary

This PR adds a new Go tool to migrate audit data from multiple tenant-specific indexes to a single unified index using tenant filtering. This enables consolidation from the pattern `audit-{tenant_id}-{date}` to a single `hermes` index with `tenant_ids` field filtering.

## Changes

### Core Migration Tool (`migrate-indexes/`)

**main.go** - Index migration implementation:
- Connects to OpenSearch/Elasticsearch
- Discovers indexes matching configurable pattern (default: `audit-*`)
- Extracts OpenStack project IDs from index names (strips date suffixes)
- Adds `tenant_ids` keyword array field to each document
- Bulk copies documents to target index (default: `hermes`)
- Uses scroll API for efficient large dataset handling

**Key Features:**
- Configurable OpenSearch host, index pattern, target index, and prefix
- Dry-run mode for preview without making changes
- Batch processing (1000 docs/batch) with 2 parallel workers
- Automatic date suffix removal for OpenStack compatibility
- Preserves original document IDs and fields

### Tenant ID Extraction

The tool intelligently extracts OpenStack project IDs by removing date suffixes:

- `audit-tenant123-2024.01` → `tenant_ids: ["tenant123"]`
- `audit-abc-def-123-6-2024.11` → `tenant_ids: ["abc-def-123"]`
- `audit-project456-2025.03` → `tenant_ids: ["project456"]`

Pattern matching removes:
- `-YYYY.MM` (date suffix)
- `-N-YYYY.MM` (version + date suffix, where N is 1-2 digits)

### Testing Infrastructure

**docker-compose.yml** - Lightweight OpenSearch test environment:
- OpenSearch 2.11.0 single-node instance
- 512MB heap (minimal footprint)
- Security disabled for easier testing
- Health checks configured

**main_test.go** - Comprehensive test suite:
- **Unit Tests:** 11 test cases for tenant ID extraction
- **Integration Tests:**
  - Index pattern matching
  - Document migration with tenant_ids injection
  - End-to-end multi-index migration workflow
  - Tenant filtering validation

**Test Automation:**
- `Makefile` - Convenient test targets
- `run-tests.sh` - Interactive test script with colored output
- Both support Docker Compose v2 syntax

### Documentation

**README.md** - Complete usage documentation:
- Installation and usage instructions
- Command-line options and examples
- Migration process explanation with examples
- Testing guide (unit + integration tests)
- Performance characteristics
- Docker setup instructions

## Usage

```bash
cd migrate-indexes

# Preview migration (dry run)
go run main.go -elastichost http://opensearch:9200 -dry-run

# Run actual migration
go run main.go -elastichost http://opensearch:9200

# Custom target index
go run main.go -elastichost http://opensearch:9200 -target my-unified-index
```

## Testing

```bash
# Run all tests (starts Docker automatically)
make test

# Unit tests only (no Docker required)
make test-unit

# Integration tests only
make docker-up
make test-integration
```

All tests pass successfully.

## Benefits

1. **Simplified Index Management** - Single index instead of hundreds of tenant-specific indexes
2. **Efficient Filtering** - Keyword array field enables fast tenant-based queries
3. **Scalability** - Reduces index overhead and improves cluster performance
4. **OpenStack Compatible** - Properly extracts project IDs without date artifacts
5. **Safe Migration** - Dry-run mode and comprehensive testing
6. **Flexible Configuration** - Customizable patterns and target indexes

## Commits

- `84c3314` - Add OpenSearch index migration tool
- `eccc7e5` - Add comprehensive tests for index migration tool
- `546cf25` - Strip date suffixes from tenant IDs for OpenStack compatibility

## Future Enhancements

- Support for multiple tenant IDs per document (already array-based)
- Progress tracking and resume capability
- Incremental migration support
- Custom field mapping options

---

## How to Create the PR

Visit: https://github.com/notque/elktools/compare/master...claude/opensearch-index-copy-01RUfwRWoLTRr5MJ2QqoqB5T

Or use the GitHub CLI:
```bash
gh pr create --base master --head claude/opensearch-index-copy-01RUfwRWoLTRr5MJ2QqoqB5T --title "Add OpenSearch index migration tool for tenant consolidation"
```
