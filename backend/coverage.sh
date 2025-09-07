#!/bin/bash
# Coverage verification script with 100% threshold enforcement

set -e

echo "🧪 Running Go tests with coverage collection..."

# Run tests with coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Generate text coverage report
echo "📊 Generating coverage report..."
go tool cover -func=coverage.out > coverage.txt

# Display coverage summary
echo "📋 Coverage Summary:"
cat coverage.txt

# Extract total coverage percentage
COVERAGE=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | sed 's/%//')
THRESHOLD=100.0

echo ""
echo "🎯 Coverage Threshold: ${THRESHOLD}%"
echo "📊 Actual Coverage: ${COVERAGE}%"

# Check if coverage meets threshold
if awk "BEGIN {exit !($COVERAGE >= $THRESHOLD)}"; then
    echo "✅ SUCCESS: Coverage ${COVERAGE}% meets the required ${THRESHOLD}% threshold!"
    echo "🎉 All code is properly tested!"
else
    echo "❌ FAILURE: Coverage ${COVERAGE}% is below the required ${THRESHOLD}% threshold!"
    echo ""
    echo "🔍 Files with less than 100% coverage:"
    go tool cover -func=coverage.out | grep -v "100.0%" | grep -v "total:" || true
    echo ""
    echo "💡 Please add tests to cover all code paths."
    exit 1
fi

echo ""
echo "🎯 Perfect! 100% coverage achieved!"