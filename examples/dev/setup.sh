#!/bin/bash
# FTUE Setup Script - Automates first-time user experience
set -e

API_URL="${API_URL:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@test.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-password123}"
ADMIN_NAME="${ADMIN_NAME:-Admin User}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Wait for API to be ready
wait_for_api() {
    log_info "Waiting for API to be ready..."
    for i in {1..30}; do
        if curl -s "$API_URL/health" > /dev/null 2>&1; then
            log_info "API is ready"
            return 0
        fi
        sleep 2
    done
    log_error "API not ready after 60 seconds"
    exit 1
}

# Step 1: Create admin account
create_admin() {
    log_info "Step 1: Creating admin account..."

    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/setup" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\",\"name\":\"$ADMIN_NAME\"}")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "200" ]; then
        log_info "Admin account created: $ADMIN_EMAIL"
    elif [ "$HTTP_CODE" = "409" ] || [ "$HTTP_CODE" = "403" ]; then
        log_warn "Admin account already exists"
    else
        log_error "Failed to create admin: $BODY"
        exit 1
    fi
}

# Step 2: Login and get token
login() {
    log_info "Step 2: Logging in..."

    RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")

    TOKEN=$(echo "$RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$TOKEN" ]; then
        log_error "Failed to login: $RESPONSE"
        exit 1
    fi

    log_info "Login successful"
    echo "$TOKEN"
}

# Step 3: Connect Vespa
connect_vespa() {
    local TOKEN=$1
    log_info "Step 3: Connecting Vespa..."

    # Wait for Vespa config server
    log_info "Waiting for Vespa config server..."
    for i in {1..30}; do
        if curl -s "http://localhost:19071/state/v1/health" > /dev/null 2>&1; then
            break
        fi
        sleep 2
    done

    RESPONSE=$(curl -s -X POST "$API_URL/api/v1/admin/vespa/connect" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d '{"endpoint":"http://vespa:19071","dev_mode":true}')

    CONNECTED=$(echo "$RESPONSE" | grep -o '"connected":true')
    HEALTHY=$(echo "$RESPONSE" | grep -o '"healthy":true')

    if [ -n "$CONNECTED" ] && [ -n "$HEALTHY" ]; then
        log_info "Vespa connected and healthy"
        SCHEMA_MODE=$(echo "$RESPONSE" | grep -o '"schema_mode":"[^"]*"' | cut -d'"' -f4)
        log_info "Schema mode: $SCHEMA_MODE"
    else
        log_error "Failed to connect Vespa: $RESPONSE"
        exit 1
    fi
}

# Step 4: Verify health
verify_health() {
    log_info "Step 4: Verifying system health..."

    RESPONSE=$(curl -s "$API_URL/health")
    STATUS=$(echo "$RESPONSE" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ "$STATUS" = "healthy" ]; then
        log_info "All systems healthy"
        echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
    else
        log_warn "System status: $STATUS"
        echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
    fi
}

# Main
main() {
    echo "========================================"
    echo "  Sercha FTUE Setup Script"
    echo "========================================"
    echo ""

    wait_for_api
    create_admin
    TOKEN=$(login)
    connect_vespa "$TOKEN"
    verify_health

    echo ""
    echo "========================================"
    log_info "Setup complete!"
    echo ""
    echo "  API:      $API_URL"
    echo "  Email:    $ADMIN_EMAIL"
    echo "  Password: $ADMIN_PASSWORD"
    echo ""
    echo "  Try LocalFS: See README.md for usage"
    echo "========================================"
}

main "$@"
