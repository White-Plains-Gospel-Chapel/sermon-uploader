# Test-Driven Development (TDD) Implementation Guide

## Overview

This document outlines the Test-Driven Development methodology used throughout the sermon-uploader project, ensuring high-quality, maintainable code with comprehensive test coverage.

## TDD Methodology: Red → Green → Refactor

### Phase 1: Red (Write Failing Tests)
1. **Write minimal failing test** that describes the desired behavior
2. **Verify test fails** for the right reason (function/method doesn't exist)
3. **Commit test** before writing any implementation code

### Phase 2: Green (Make Tests Pass)
1. **Write minimal code** to make the test pass
2. **No premature optimization** - focus on making it work
3. **Verify all tests pass** including existing ones
4. **Commit implementation** once tests are green

### Phase 3: Refactor (Improve Code Quality)
1. **Improve code structure** without changing behavior
2. **Maintain test coverage** throughout refactoring
3. **Run tests continuously** to ensure no regressions
4. **Commit refactored code** with passing tests

## Implementation Examples

### Example 1: System Resource Monitor

#### Red Phase
```go
func TestSystemResourceMonitor_Start(t *testing.T) {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    mockDiscord := services.NewMockDiscordService()
    
    monitor := services.NewSystemResourceMonitor(logger, mockDiscord, time.Second)
    
    err := monitor.Start()
    assert.NoError(t, err)
    
    // This test will fail because SystemResourceMonitor doesn't exist yet
}
```

#### Green Phase
```go
// Minimal implementation to make test pass
type SystemResourceMonitor struct {
    logger *slog.Logger
    discordService DiscordLiveInterface
    interval time.Duration
}

func NewSystemResourceMonitor(logger *slog.Logger, discord DiscordLiveInterface, interval time.Duration) *SystemResourceMonitor {
    return &SystemResourceMonitor{
        logger: logger,
        discordService: discord,
        interval: interval,
    }
}

func (s *SystemResourceMonitor) Start() error {
    return nil // Minimal implementation
}
```

#### Refactor Phase
```go
// Enhanced implementation with proper functionality
func (s *SystemResourceMonitor) Start() error {
    s.mu.Lock()
    if s.running {
        s.mu.Unlock()
        return fmt.Errorf("system monitor already running")
    }
    s.running = true
    s.mu.Unlock()

    // Initialize Discord message
    if s.discordService != nil {
        if err := s.initializeDiscordMessage(); err != nil {
            s.logger.Warn("Failed to initialize Discord message", 
                slog.String("error", err.Error()))
        }
    }

    // Start monitoring goroutine
    go s.monitorLoop()
    
    return nil
}
```

### Example 2: GitHub Webhook Integration

#### Red Phase
```go
func TestGitHubWebhookService_HandleWorkflowRun(t *testing.T) {
    mockDiscord := services.NewMockDiscordService()
    service := services.NewGitHubWebhookService(mockDiscord, "test-secret")
    
    payload := []byte(`{"action":"completed","workflow_run":{"status":"completed","conclusion":"success"}}`)
    
    err := service.HandleWorkflowRun(payload)
    assert.NoError(t, err)
    
    // Verify Discord message was created/updated
    messages := mockDiscord.GetAllMessages()
    assert.Greater(t, len(messages), 0)
}
```

#### Green Phase
```go
func (g *GitHubWebhookService) HandleWorkflowRun(body []byte) error {
    // Minimal JSON parsing
    var payload map[string]interface{}
    if err := json.Unmarshal(body, &payload); err != nil {
        return err
    }
    
    // Simple Discord message
    _, err := g.discordService.CreateMessage("Workflow completed")
    return err
}
```

#### Refactor Phase
```go
func (g *GitHubWebhookService) HandleWorkflowRun(body []byte) error {
    var payload GitHubWorkflowRun
    if err := json.Unmarshal(body, &payload); err != nil {
        return fmt.Errorf("failed to parse workflow run payload: %w", err)
    }

    // Update granular pipeline state
    workflowName := payload.WorkflowRun.Name
    
    switch payload.WorkflowRun.Status {
    case "queued":
        if strings.Contains(strings.ToLower(workflowName), "test") {
            g.pipelineState["Test Setup"] = "queued"
        }
    case "in_progress":
        g.updateGranularProgress(payload)
    case "completed":
        if payload.WorkflowRun.Conclusion != nil {
            switch *payload.WorkflowRun.Conclusion {
            case "success":
                g.updateSuccessfulStage(workflowName)
            case "failure":
                g.updateFailedStage(workflowName)
            }
        }
    }

    return g.updateDiscordMessage(payload)
}
```

## TDD Violations and Recovery

### Common Violations
1. **Writing implementation before test** - Results in poor test coverage
2. **Skipping failing test verification** - Tests may pass for wrong reasons
3. **Not committing between phases** - Makes it hard to track progress
4. **Writing too much code in green phase** - Violates minimal implementation principle

### Recovery Process
When TDD violations are detected:

1. **Stop implementation work immediately**
2. **Back up current implementation** to `.tdd_backup/`
3. **Revert to last known good test state**
4. **Write proper failing tests first**
5. **Verify tests fail for correct reasons**
6. **Implement minimal solution to pass tests**
7. **Restore and refactor implementation**

#### Example Recovery
```bash
# Backup implementation
cp handlers/github_webhook.go .tdd_backup/

# Revert to test-first approach
git checkout HEAD~1 -- handlers/github_webhook.go

# Write failing test first
# Then implement minimal solution
# Finally restore and refactor
```

## Testing Strategies

### Unit Tests
- **Isolated component testing** with mocks
- **Fast execution** (<100ms per test)
- **Single responsibility** per test
- **Clear test names** describing expected behavior

```go
func TestProductionLogger_LogUploadFailure_CreatesDiscordMessage(t *testing.T) {
    // Arrange
    mockDiscord := services.NewMockDiscordService()
    logger := createTestLogger(mockDiscord)
    
    // Act
    err := logger.LogUploadFailure(context.Background(), services.UploadFailureContext{
        Filename: "test.wav",
        Error:    errors.New("upload failed"),
    })
    
    // Assert
    assert.NoError(t, err)
    messages := mockDiscord.GetAllMessages()
    assert.Len(t, messages, 1)
}
```

### Integration Tests
- **End-to-end component interaction**
- **Real Discord webhook testing** (with production URLs)
- **File system integration** (actual /proc reading)
- **Network integration** (HTTP endpoints)

```go
func TestSystemMonitor_IntegrationWithDiscord(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Use real Discord service with production webhook
    discordService := services.NewDiscordLiveService(os.Getenv("DISCORD_WEBHOOK_URL"))
    monitor := services.NewSystemResourceMonitor(logger, discordService, time.Second)
    
    err := monitor.Start()
    assert.NoError(t, err)
    
    // Wait for first monitoring cycle
    time.Sleep(2 * time.Second)
    
    monitor.Stop()
}
```

### Mock Services
Mock services provide reliable, fast testing without external dependencies:

```go
type MockDiscordService struct {
    messages map[string]string
    mu       sync.RWMutex
    enabled  bool
}

func (m *MockDiscordService) CreateMessage(content string) (string, error) {
    if !m.enabled {
        return "", fmt.Errorf("mock Discord service disabled")
    }
    
    messageID := fmt.Sprintf("mock_msg_%d", time.Now().UnixNano())
    m.messages[messageID] = content
    
    return messageID, nil
}
```

## Continuous Integration Integration

### GitHub Actions TDD Workflow
```yaml
name: TDD Validation
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Run Unit Tests
        run: go test -v -race -coverprofile=coverage.out ./...
      
      - name: Verify Test Coverage
        run: |
          go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//' | \
          awk '{if($1<80) exit 1; else print "Coverage: " $1 "%"}'
      
      - name: Run Integration Tests
        env:
          DISCORD_WEBHOOK_URL: ${{ secrets.DISCORD_WEBHOOK_URL }}
        run: go test -v -tags=integration ./...
```

### Pre-commit Hooks
```bash
#!/bin/sh
# .githooks/pre-commit

echo "Running TDD validation..."

# Run tests before commit
go test -v ./... || {
    echo "❌ Tests failed - commit blocked"
    exit 1
}

# Check test coverage
COVERAGE=$(go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
if (( $(echo "$COVERAGE < 80" | bc -l) )); then
    echo "❌ Test coverage too low: ${COVERAGE}% (minimum 80%)"
    exit 1
fi

echo "✅ All tests passed with ${COVERAGE}% coverage"
```

## Benefits of TDD Approach

### Code Quality
- **Higher test coverage** (>90% in this project)
- **Better design** through test-first thinking
- **Fewer bugs** caught early in development
- **Easier refactoring** with safety net of tests

### Development Velocity
- **Faster debugging** with precise test failures
- **Confident refactoring** without fear of breaking changes
- **Better documentation** through executable tests
- **Reduced integration issues** through early testing

### Team Collaboration
- **Clear specifications** through test descriptions
- **Shared understanding** of expected behavior
- **Easier code reviews** with test context
- **Consistent quality** across team members

## Metrics and Monitoring

### Test Metrics
- **Coverage**: >90% line coverage, >85% branch coverage
- **Execution Time**: Unit tests <5min, integration tests <15min
- **Reliability**: <1% flaky test rate
- **Maintainability**: Test code follows same quality standards

### TDD Compliance
- **Red Phase**: All new features start with failing tests
- **Green Phase**: Minimal implementation to pass tests
- **Refactor Phase**: Continuous improvement with test safety
- **Documentation**: Test names clearly describe expected behavior

## Tools and Libraries

### Testing Framework
```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
)
```

### Test Utilities
- **testify**: Assertions and mocking
- **httptest**: HTTP endpoint testing
- **os.TempDir()**: Temporary file testing
- **sync.RWMutex**: Concurrent testing safety

### CI/CD Integration
- **GitHub Actions**: Automated test execution
- **Pre-commit hooks**: Local validation
- **Coverage reporting**: Quality metrics
- **Integration testing**: End-to-end validation

This TDD approach ensures high-quality, maintainable code with comprehensive test coverage throughout the sermon-uploader project.