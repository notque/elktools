#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting test environment...${NC}"

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed or not in PATH${NC}"
    echo "Please install Docker to run integration tests"
    exit 1
fi

# Start Docker Compose
echo "Starting OpenSearch container..."
docker compose up -d

# Wait for OpenSearch to be ready
echo "Waiting for OpenSearch to be ready..."
max_attempts=30
attempt=0

while [ $attempt -lt $max_attempts ]; do
    if curl -s http://localhost:9200/_cluster/health > /dev/null 2>&1; then
        echo -e "${GREEN}OpenSearch is ready!${NC}"
        break
    fi
    attempt=$((attempt + 1))
    echo "Waiting... (attempt $attempt/$max_attempts)"
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo -e "${RED}Timeout waiting for OpenSearch to start${NC}"
    docker compose logs opensearch
    exit 1
fi

# Get cluster info
echo -e "\n${GREEN}Cluster Info:${NC}"
curl -s http://localhost:9200 | head -n 20

# Run unit tests
echo -e "\n${GREEN}Running unit tests...${NC}"
go test -v -run "^TestExtractTenantID$" -count=1

# Run integration tests
echo -e "\n${GREEN}Running integration tests...${NC}"
go test -v -run "^TestIntegration$" -count=1 -timeout 5m

# Test result
if [ $? -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed!${NC}"
else
    echo -e "\n${RED}Tests failed!${NC}"
    exit 1
fi

# Ask if user wants to stop the container
echo -e "\n${YELLOW}Do you want to stop the OpenSearch container? (y/N)${NC}"
read -r response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    echo "Stopping containers..."
    docker compose down -v
    echo -e "${GREEN}Cleanup complete!${NC}"
else
    echo -e "${YELLOW}OpenSearch is still running at http://localhost:9200${NC}"
    echo "Run 'docker compose down -v' to stop and remove it"
fi
