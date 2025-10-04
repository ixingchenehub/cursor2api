#!/usr/bin/env bash
#
# cursor2api startup script for Linux/macOS
# This script cleans cache, builds, and starts the server
#

set -e  # Exit immediately if a command exits with a non-zero status
set -u  # Treat unset variables as an error

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Print colored message
print_info() {
    echo -e "${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_error() {
    echo -e "${RED}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}$1${NC}"
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Main script starts here
main() {
    print_info "========================================"
    print_info "  cursor2api Server Startup Script"
    print_info "========================================"
    echo ""

    # Check prerequisites
    if ! command_exists go; then
        print_error "ERROR: Go is not installed or not in PATH"
        print_info "Please install Go from https://golang.org/dl/"
        exit 1
    fi

    # Step 1: Clean Go cache and build artifacts
    print_info "[Step 1/4] Cleaning Go cache and build artifacts..."
    echo ""

    # Clean Go build cache
    if ! go clean -cache; then
        print_error "ERROR: Failed to clean Go cache"
        exit 1
    fi

    # Clean Go module cache
    if ! go clean -modcache; then
        print_error "ERROR: Failed to clean Go module cache"
        exit 1
    fi

    # Remove binary files if they exist
    if [ -f "cursor2api" ]; then
        print_info "Removing old binary: cursor2api"
        rm -f cursor2api
    fi

    if [ -f "bin/cursor2api" ]; then
        print_info "Removing old binary: bin/cursor2api"
        rm -f bin/cursor2api
    fi

    print_success "✓ Cache cleaned successfully"
    echo ""

    # Step 2: Download dependencies
    print_info "[Step 2/4] Downloading Go dependencies..."
    echo ""

    if ! go mod download; then
        print_error "ERROR: Failed to download dependencies"
        exit 1
    fi

    print_success "✓ Dependencies downloaded"
    echo ""

    # Step 3: Build the project
    print_info "[Step 3/4] Building cursor2api..."
    echo ""

    if ! go build -v -o cursor2api main.go; then
        print_error "ERROR: Build failed"
        exit 1
    fi

    print_success "✓ Build completed successfully"
    echo ""

    # Step 4: Run the server
    print_info "[Step 4/4] Starting server..."
    echo ""

    # Check if .env file exists
    if [ ! -f ".env" ]; then
        print_warning "WARNING: .env file not found"
        print_info "Please create .env file based on .env.example"
        exit 1
    fi

    print_info "========================================"
    print_info "  Starting cursor2api Server"
    print_info "========================================"
    echo ""

    # Run the built binary
    # The Go application will load .env file automatically
    ./cursor2api
}

# Execute main function
main