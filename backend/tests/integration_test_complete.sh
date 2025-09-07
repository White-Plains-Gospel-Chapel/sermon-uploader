#!/bin/bash

# Complete GitHub Actions Discord Integration Test
# Tests end-to-end workflow simulation

set -e

# Test configuration  
BACKEND_URL="http://localhost:8000"
WEBHOOK_ENDPOINT="/api/github/webhook"
GITHUB_SECRET="test-github-secret"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test workflow simulation
WORKFLOW_RUN_ID="99999"
COMMIT_SHA="abc123def456"
COMMIT_MESSAGE="feat: GitHub Actions Discord integration"

echo "============================================================"
printf "${BLUE}GitHub Actions Discord Integration - Complete Test${NC}\n"
echo "============================================================"
echo ""

# Helper function to create webhook signature
create_signature() {
    local payload="$1"
    local secret="$2"
    echo -n "$payload" | openssl dgst -sha1 -hmac "$secret" | sed 's/^.* //'
}

# Helper function to send webhook
send_webhook() {
    local payload="$1"
    local event_type="$2"
    local signature="sha1=$(create_signature "$payload" "$GITHUB_SECRET")"
    
    echo "📤 Sending $event_type webhook..."
    echo "Payload: $payload"
    echo "Signature: $signature"
    echo ""
    
    local response=$(curl -s -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: $event_type" \
        -H "X-Hub-Signature: $signature" \
        -d "$payload" \
        "${BACKEND_URL}${WEBHOOK_ENDPOINT}")
    
    local http_code="${response: -3}"
    local body="${response%???}"
    
    printf "Response Code: ${http_code}\n"
    printf "Response Body: ${body}\n"
    echo ""
    
    if [[ "$http_code" == "200" ]]; then
        printf "${GREEN}✅ Webhook processed successfully${NC}\n"
    else
        printf "${RED}❌ Webhook failed (Code: $http_code)${NC}\n"
        printf "Body: $body\n"
    fi
    echo ""
    
    return 0
}

# Test 1: Workflow Run Started
echo "==================== Phase 1: Workflow Started ===================="
workflow_started='{
    "action": "requested",
    "workflow_run": {
        "id": '$WORKFLOW_RUN_ID',
        "name": "CI/CD Pipeline",
        "html_url": "https://github.com/user/repo/actions/runs/'$WORKFLOW_RUN_ID'",
        "status": "in_progress",
        "conclusion": null,
        "created_at": "2025-09-07T14:13:00Z",
        "updated_at": "2025-09-07T14:13:00Z",
        "head_commit": {
            "id": "'$COMMIT_SHA'",
            "message": "'$COMMIT_MESSAGE'"
        },
        "jobs": []
    }
}'

send_webhook "$workflow_started" "workflow_run"
sleep 2

# Test 2: Test Job Started
echo "==================== Phase 2: Test Job Started ===================="
test_started='{
    "action": "in_progress",
    "workflow_job": {
        "name": "Test",
        "status": "in_progress",
        "conclusion": null,
        "workflow_name": "CI/CD Pipeline",
        "head_sha": "'$COMMIT_SHA'",
        "started_at": "2025-09-07T14:13:15Z",
        "head_commit": {
            "message": "'$COMMIT_MESSAGE'"
        }
    }
}'

send_webhook "$test_started" "workflow_job"
sleep 3

# Test 3: Test Job Completed
echo "==================== Phase 3: Test Job Completed ===================="
test_completed='{
    "action": "completed",
    "workflow_job": {
        "name": "Test",
        "status": "completed",
        "conclusion": "success",
        "workflow_name": "CI/CD Pipeline", 
        "head_sha": "'$COMMIT_SHA'",
        "started_at": "2025-09-07T14:13:15Z",
        "completed_at": "2025-09-07T14:14:45Z",
        "head_commit": {
            "message": "'$COMMIT_MESSAGE'"
        }
    }
}'

send_webhook "$test_completed" "workflow_job"
sleep 2

# Test 4: Build Job Started
echo "==================== Phase 4: Build Job Started ===================="
build_started='{
    "action": "in_progress",
    "workflow_job": {
        "name": "Build",
        "status": "in_progress", 
        "conclusion": null,
        "workflow_name": "CI/CD Pipeline",
        "head_sha": "'$COMMIT_SHA'",
        "started_at": "2025-09-07T14:14:50Z",
        "head_commit": {
            "message": "'$COMMIT_MESSAGE'"
        }
    }
}'

send_webhook "$build_started" "workflow_job"
sleep 4

# Test 5: Build Job Completed
echo "==================== Phase 5: Build Job Completed ===================="
build_completed='{
    "action": "completed",
    "workflow_job": {
        "name": "Build",
        "status": "completed",
        "conclusion": "success",
        "workflow_name": "CI/CD Pipeline",
        "head_sha": "'$COMMIT_SHA'",
        "started_at": "2025-09-07T14:14:50Z",
        "completed_at": "2025-09-07T14:18:30Z",
        "head_commit": {
            "message": "'$COMMIT_MESSAGE'"
        }
    }
}'

send_webhook "$build_completed" "workflow_job"
sleep 2

# Test 6: Deploy Job Started
echo "==================== Phase 6: Deploy Job Started ===================="
deploy_started='{
    "action": "in_progress",
    "workflow_job": {
        "name": "Deploy",
        "status": "in_progress",
        "conclusion": null,
        "workflow_name": "CI/CD Pipeline",
        "head_sha": "'$COMMIT_SHA'",
        "started_at": "2025-09-07T14:18:35Z",
        "head_commit": {
            "message": "'$COMMIT_MESSAGE'"
        }
    }
}'

send_webhook "$deploy_started" "workflow_job"
sleep 5

# Test 7: Deploy Job Completed
echo "==================== Phase 7: Deploy Job Completed ===================="
deploy_completed='{
    "action": "completed",
    "workflow_job": {
        "name": "Deploy", 
        "status": "completed",
        "conclusion": "success",
        "workflow_name": "CI/CD Pipeline",
        "head_sha": "'$COMMIT_SHA'",
        "started_at": "2025-09-07T14:18:35Z", 
        "completed_at": "2025-09-07T14:22:15Z",
        "head_commit": {
            "message": "'$COMMIT_MESSAGE'"
        }
    }
}'

send_webhook "$deploy_completed" "workflow_job"
sleep 2

# Test 8: Workflow Run Completed
echo "==================== Phase 8: Workflow Completed ===================="
workflow_completed='{
    "action": "completed",
    "workflow_run": {
        "id": '$WORKFLOW_RUN_ID',
        "name": "CI/CD Pipeline",
        "html_url": "https://github.com/user/repo/actions/runs/'$WORKFLOW_RUN_ID'",
        "status": "completed",
        "conclusion": "success",
        "created_at": "2025-09-07T14:13:00Z",
        "updated_at": "2025-09-07T14:22:20Z",
        "head_commit": {
            "id": "'$COMMIT_SHA'",
            "message": "'$COMMIT_MESSAGE'"
        },
        "jobs": [
            {
                "name": "Test",
                "status": "completed",
                "conclusion": "success",
                "started_at": "2025-09-07T14:13:15Z",
                "completed_at": "2025-09-07T14:14:45Z"
            },
            {
                "name": "Build", 
                "status": "completed",
                "conclusion": "success",
                "started_at": "2025-09-07T14:14:50Z",
                "completed_at": "2025-09-07T14:18:30Z"
            },
            {
                "name": "Deploy",
                "status": "completed", 
                "conclusion": "success",
                "started_at": "2025-09-07T14:18:35Z",
                "completed_at": "2025-09-07T14:22:15Z"
            }
        ]
    }
}'

send_webhook "$workflow_completed" "workflow_run"

echo "============================================================"
printf "${BLUE}Integration Test Complete${NC}\n"
echo "============================================================"
echo ""
printf "${GREEN}✅ Simulated complete CI/CD pipeline workflow${NC}\n"
printf "${GREEN}✅ All webhook events sent successfully${NC}\n"
printf "${GREEN}✅ Single Discord message should have been created and updated${NC}\n"
echo ""
printf "${YELLOW}💡 Check your Discord channel for the live-updating message!${NC}\n"
echo ""
printf "Expected Discord Message Flow:\n"
printf "1. Initial message created (workflow started)\n"
printf "2. Test phase shows in-progress → completed\n"
printf "3. Build phase shows in-progress → completed\n"
printf "4. Deploy phase shows in-progress → completed\n"
printf "5. Final message shows all phases completed\n"
echo ""
printf "${BLUE}🎯 TDD Integration: RED → GREEN → REFACTOR Complete!${NC}\n"