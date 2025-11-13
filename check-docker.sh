#!/bin/bash

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "================================"
echo "Docker Environment Check"
echo "================================"
echo ""

# Check if Docker is installed
echo -n "Checking if Docker is installed... "
if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓ Docker found${NC}"
else
    echo -e "${RED}✗ Docker not found${NC}"
    echo ""
    echo "Docker is not installed. Please install Docker Desktop:"
    echo "  Windows/macOS: https://www.docker.com/products/docker-desktop/"
    echo "  Linux: https://docs.docker.com/engine/install/"
    exit 1
fi

# Check if Docker daemon is running
echo -n "Checking if Docker daemon is running... "
if docker ps &> /dev/null; then
    echo -e "${GREEN}✓ Docker daemon is running${NC}"
else
    echo -e "${RED}✗ Docker daemon is not running${NC}"
    echo ""
    echo "Docker is installed but not running. Please:"
    echo "  1. Start Docker Desktop (Windows/macOS)"
    echo "  2. Or start Docker daemon (Linux): sudo systemctl start docker"
    exit 1
fi

# Check Docker version
echo -n "Docker version: "
docker version --format '{{.Server.Version}}' 2>/dev/null
echo ""

# Try to pull Redis image
echo -n "Checking if Redis image is available... "
if docker images redis:7-alpine --format "{{.Repository}}" 2>/dev/null | grep -q redis; then
    echo -e "${GREEN}✓ Redis image already downloaded${NC}"
else
    echo -e "${YELLOW}! Redis image not found${NC}"
    echo -n "Pulling Redis image... "
    if docker pull redis:7-alpine &> /dev/null; then
        echo -e "${GREEN}✓ Redis image downloaded${NC}"
    else
        echo -e "${RED}✗ Failed to pull Redis image${NC}"
        echo "Please check your internet connection"
        exit 1
    fi
fi

# Test creating a container
echo -n "Testing Docker container creation... "
TEST_CONTAINER="redis-test-$$"
if docker run -d --name "$TEST_CONTAINER" --rm redis:7-alpine &> /dev/null; then
    echo -e "${GREEN}✓ Container creation successful${NC}"
    docker stop "$TEST_CONTAINER" &> /dev/null
else
    echo -e "${RED}✗ Failed to create container${NC}"
    exit 1
fi

echo ""
echo "================================"
echo -e "${GREEN}All checks passed!${NC}"
echo "================================"
echo ""
echo "You can now run the tests:"
echo "  go test -v ./..."
echo "  make test"
echo ""

exit 0
