#!/bin/bash

set -e

echo "🚀 Starting Amazon Orders Service Deployment"
echo "============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# Check if .env exists
if [ ! -f .env ]; then
    print_warning ".env file not found, creating from .env.docker"
    cp .env.docker .env
    print_warning "Please edit .env with your actual credentials!"
    exit 1
fi

# Check Docker
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed. Please install Docker first."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    print_error "Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

print_success "Docker and Docker Compose found"

# Load environment variables
export $(cat .env | grep -v '^#' | xargs)

# Parse command line arguments
COMMAND=${1:-start}
PROFILE=${2:-}

case $COMMAND in
    start)
        echo "Starting services..."
        if [ "$PROFILE" = "production" ]; then
            # Check SSL certificates for production
            if [ ! -f nginx/ssl/cert.pem ] || [ ! -f nginx/ssl/key.pem ]; then
                print_warning "SSL certificates not found. Generating self-signed certificates..."
                cd nginx && ./generate-ssl.sh && cd ..
            fi
            docker-compose --profile production up -d
            print_success "Services started with Nginx (production mode)"
        else
            docker-compose up -d postgres app
            print_success "Services started (development mode)"
        fi
        ;;
        
    stop)
        echo "Stopping services..."
        docker-compose stop
        print_success "Services stopped"
        ;;
        
    restart)
        echo "Restarting services..."
        docker-compose restart
        print_success "Services restarted"
        ;;
        
    logs)
        docker-compose logs -f
        ;;
        
    status)
        docker-compose ps
        ;;
        
    build)
        echo "Building application..."
        docker-compose build app
        print_success "Application built"
        ;;
        
    update)
        echo "Updating application..."
        docker-compose build app
        docker-compose up -d app
        print_success "Application updated"
        ;;
        
    backup)
        BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql"
        echo "Creating database backup: $BACKUP_FILE"
        docker-compose exec -T postgres pg_dump -U ${DB_USER:-postgres} ${DB_NAME:-amz_orders} > $BACKUP_FILE
        print_success "Backup created: $BACKUP_FILE"
        ;;
        
    restore)
        if [ -z "$2" ]; then
            print_error "Usage: $0 restore <backup_file>"
            exit 1
        fi
        BACKUP_FILE=$2
        if [ ! -f "$BACKUP_FILE" ]; then
            print_error "Backup file not found: $BACKUP_FILE"
            exit 1
        fi
        echo "Restoring database from: $BACKUP_FILE"
        docker-compose exec -T postgres psql -U ${DB_USER:-postgres} ${DB_NAME:-amz_orders} < $BACKUP_FILE
        print_success "Database restored"
        ;;
        
    clean)
        echo "Cleaning up..."
        docker-compose down -v
        print_success "All services stopped and volumes removed"
        ;;
        
    health)
        echo "Checking service health..."
        if curl -s -o /dev/null -w "%{http_code}" http://localhost:${APP_PORT:-8080}/health | grep -q "200"; then
            print_success "Service is healthy"
        else
            print_error "Service is not responding"
            exit 1
        fi
        ;;
        
    test)
        echo "Testing service..."
        
        # Health check
        echo -n "Health check: "
        if curl -s http://localhost:${APP_PORT:-8080}/health | grep -q "ok"; then
            print_success "Passed"
        else
            print_error "Failed"
            exit 1
        fi
        
        # Import sample data
        echo -n "Import sample data: "
        RESPONSE=$(curl -s -X POST http://localhost:${APP_PORT:-8080}/api/v1/orders/import-sample \
            -H "Content-Type: application/json" \
            -d '{"file_path":"./sample_orders.json"}')
        
        if echo $RESPONSE | grep -q "success"; then
            print_success "Passed"
        else
            print_error "Failed"
            echo $RESPONSE
            exit 1
        fi
        
        # List orders
        echo -n "List orders: "
        RESPONSE=$(curl -s http://localhost:${APP_PORT:-8080}/api/v1/orders?page=1&limit=10)
        if echo $RESPONSE | grep -q "data"; then
            print_success "Passed"
        else
            print_error "Failed"
            echo $RESPONSE
            exit 1
        fi
        
        print_success "All tests passed!"
        ;;
        
    *)
        echo "Amazon Orders Service Deployment Script"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  start [production]  - Start services (add 'production' for Nginx)"
        echo "  stop                - Stop services"
        echo "  restart             - Restart services"
        echo "  logs                - View logs"
        echo "  status              - Show service status"
        echo "  build               - Build application"
        echo "  update              - Update and restart application"
        echo "  backup              - Backup database"
        echo "  restore <file>      - Restore database from backup"
        echo "  clean               - Stop services and remove volumes"
        echo "  health              - Check service health"
        echo "  test                - Run basic tests"
        echo ""
        echo "Examples:"
        echo "  $0 start            - Start in development mode"
        echo "  $0 start production - Start with Nginx reverse proxy"
        echo "  $0 backup           - Create database backup"
        echo "  $0 test             - Run tests"
        exit 1
        ;;
esac

exit 0
