#!/usr/bin/env bash
# ============================================================================
# One-Click Startup Script for Cross-Platform Deployment
# ============================================================================
# Description: Production-ready startup script with comprehensive checks
# Platform: Linux, macOS, WSL (Windows Subsystem for Linux)
# Usage: ./run.sh [dev|prod|stop|restart|logs|status]
# ============================================================================

set -euo pipefail

# Color output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

# Project configuration
readonly PROJECT_NAME="cursor2api"

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Check if command exists
command_exists() { command -v "$1" >/dev/null 2>&1; }

# Check Docker
check_docker() {
    log_info "Checking Docker..."
    if ! command_exists docker; then
        log_error "Docker not installed. Visit: https://docs.docker.com/get-docker/"
        return 1
    fi
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker daemon not running"
        return 1
    fi
    log_success "Docker is ready"
    return 0
}

# Check Docker Compose
check_docker_compose() {
    log_info "Checking Docker Compose..."
    if docker compose version >/dev/null 2>&1 || command_exists docker-compose; then
        log_success "Docker Compose is ready"
        return 0
    fi
    log_error "Docker Compose not installed"
    return 1
}

# Check .env file
check_env_file() {
    log_info "Checking environment configuration..."
    if [[ ! -f .env ]]; then
        if [[ -f .env.example ]]; then
            cp .env.example .env
            log_warning "Created .env from template. Please configure API_KEY!"
            log_info "Generate: echo \"sk-\$(openssl rand -base64 64 | tr -d '=+/' | cut -c1-40)\""
            return 1
        fi
        log_error ".env file missing"
        return 1
    fi
    if grep -q "^API_KEY=CHANGE_ME" .env 2>/dev/null || ! grep -q "^API_KEY=sk-" .env 2>/dev/null; then
        log_error "API_KEY invalid or not configured"
        log_info "Generate: echo \"sk-\$(openssl rand -base64 64 | tr -d '=+/' | cut -c1-40)\""
        return 1
    fi
    log_success "Environment configuration valid"
    return 0
}

# Get compose command
get_compose_cmd() {
    if docker compose version >/dev/null 2>&1; then
        echo "docker compose"
    else
        echo "docker-compose"
    fi
}

# Start services
start_services() {
    local mode="${1:-prod}"
    log_info "Starting services in $mode mode..."
    local compose_cmd=$(get_compose_cmd)
    
    [[ "$mode" == "dev" ]] && $compose_cmd build --no-cache
    
    $compose_cmd up -d
    sleep 3
    
    if $compose_cmd ps | grep -q "Up"; then
        log_success "Services started successfully"
        show_status
    else
        log_error "Failed to start services"
        $compose_cmd logs --tail=50
        return 1
    fi
}

# Stop services
stop_services() {
    log_info "Stopping services..."
    $(get_compose_cmd) down
    log_success "Services stopped"
}

# Show status
show_status() {
    local compose_cmd=$(get_compose_cmd)
    local port=$(grep -oP '^PORT=\K\d+' .env 2>/dev/null || echo "8000")
    
    echo ""
    $compose_cmd ps
    echo ""
    log_success "Service running at:"
    echo "  📍 http://localhost:${port}"
    echo ""
}

# Show logs
show_logs() {
    log_info "Showing logs (Ctrl+C to exit)..."
    $(get_compose_cmd) logs -f --tail=100
}

# Show usage
show_usage() {
    cat << EOF
${GREEN}${PROJECT_NAME} - One-Click Startup Script${NC}

${BLUE}Usage:${NC} ./run.sh [COMMAND]

${BLUE}Commands:${NC}
    dev        Start in development mode (rebuild)
    prod       Start in production mode (default)
    stop       Stop all services
    restart    Restart services
    logs       Show and follow logs
    status     Show service status
    help       Show this help

${BLUE}Examples:${NC}
    ./run.sh              # Start production mode
    ./run.sh dev          # Start development mode
    ./run.sh logs         # View logs

${BLUE}Configuration:${NC}
    Edit .env and set API_KEY
    Generate: echo "sk-\$(openssl rand -base64 64 | tr -d '=+/' | cut -c1-40)"

EOF
}

# Main
main() {
    local command="${1:-prod}"
    
    case "$command" in
        dev|prod)
            check_docker && check_docker_compose && check_env_file && start_services "$command"
            ;;
        stop)
            stop_services
            ;;
        restart)
            stop_services && sleep 2 && check_docker && check_docker_compose && check_env_file && start_services "prod"
            ;;
        logs)
            show_logs
            ;;
        status)
            show_status
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            log_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

main "$@"