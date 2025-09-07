# Testing Guide

This project uses **Vitest** with **React Testing Library** for comprehensive testing of the React/Next.js frontend.

## Setup Overview

- **Testing Framework**: Vitest (fast, modern alternative to Jest)
- **React Testing**: @testing-library/react, @testing-library/jest-dom, @testing-library/user-event
- **Coverage**: @vitest/coverage-v8 with 100% threshold
- **Environment**: jsdom for DOM simulation

## Available Commands

```bash
# Run tests once
npm test

# Run tests in watch mode
npm run test:watch

# Run tests with coverage report
npm run test:coverage

# Run tests with UI (optional)
npm run test:ui
```

## Test Structure

```
__tests__/
├── components/
│   └── upload/
│       ├── UploadManagerOptimized.test.tsx
│       └── UploadDropzone.test.tsx
└── hooks/
    └── useUploadQueueOptimized.test.ts
```

## Testing Philosophy

### React Testing Library Best Practices

1. **Test User Interactions**: Focus on what users can see and do
2. **Avoid Implementation Details**: Test behavior, not internal state
3. **Use Semantic Queries**: Prefer `getByRole`, `getByText` over `getByTestId`
4. **User Events**: Use `@testing-library/user-event` for realistic interactions

### Example Test Patterns

#### Component Testing
```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

it('handles file upload', async () => {
  const user = userEvent.setup()
  const mockOnUpload = vi.fn()
  
  render(<UploadComponent onUpload={mockOnUpload} />)
  
  const fileInput = screen.getByLabelText(/upload/i)
  const file = new File(['content'], 'test.wav', { type: 'audio/wav' })
  
  await user.upload(fileInput, file)
  
  expect(mockOnUpload).toHaveBeenCalledWith([file])
})
```

#### Hook Testing
```tsx
import { renderHook, act } from '@testing-library/react'

it('manages upload queue state', async () => {
  const { result } = renderHook(() => useUploadQueue())
  
  act(() => {
    result.current.addFile(mockFile)
  })
  
  expect(result.current.files).toHaveLength(1)
})
```

### Mocking Strategy

#### External Dependencies
- **APIs**: Mock with `vi.mock()` at module level
- **Components**: Create simple mock implementations
- **Hooks**: Mock return values, test integration separately

```tsx
vi.mock('@/services/api', () => ({
  uploadFile: vi.fn(() => Promise.resolve({ success: true }))
}))
```

## Coverage Configuration

Target: **80% coverage** across all metrics:
- **Statements**: 80%
- **Branches**: 80% 
- **Functions**: 80%
- **Lines**: 80%

Coverage excludes:
- `node_modules/`
- Test files (`src/test/`, `__tests__/`)
- Type definitions (`**/*.d.ts`)
- Config files (`**/*.config.*`)
- Build outputs (`dist/`, `build/`, `.next/`)

## Test Environment Setup

### Global Test Setup (`src/test/setup.ts`)
- DOM polyfills (ResizeObserver, matchMedia)
- File API mocks (File, FileReader, DataTransfer)
- Jest DOM matchers for better assertions
- Cleanup between tests

### Configuration (`vitest.config.ts`)
- Next.js path aliases (`@/`)
- JSX support via React plugin
- jsdom environment for DOM testing
- Coverage thresholds and reporting

## Writing New Tests

### 1. Component Tests
- Test user-visible behavior
- Mock external dependencies
- Use realistic user interactions
- Verify accessibility

### 2. Hook Tests
- Test state changes and side effects
- Use `renderHook` from Testing Library
- Test error conditions
- Mock dependent services

### 3. Integration Tests
- Test component + hook interactions
- Use minimal mocking
- Focus on user workflows
- Test error handling

## Common Testing Patterns

### Drag & Drop Testing
```tsx
it('handles drag and drop', async () => {
  const mockOnDrop = vi.fn()
  render(<DropZone onDrop={mockOnDrop} />)
  
  const dropzone = screen.getByText(/drag files/i)
  const file = new File(['content'], 'test.wav')
  
  fireEvent.drop(dropzone, {
    dataTransfer: { files: [file] }
  })
  
  expect(mockOnDrop).toHaveBeenCalledWith([file])
})
```

### Progress Testing
```tsx
it('shows upload progress', async () => {
  const { result } = renderHook(() => useUpload())
  
  act(() => {
    result.current.upload(mockFile, (progress) => {
      // Progress callback testing
    })
  })
  
  await waitFor(() => {
    expect(result.current.progress).toBeGreaterThan(0)
  })
})
```

### Error Testing
```tsx
it('handles upload errors', async () => {
  vi.mocked(api.upload).mockRejectedValue(new Error('Upload failed'))
  
  const { result } = renderHook(() => useUpload())
  
  await act(async () => {
    await expect(result.current.upload(mockFile)).rejects.toThrow('Upload failed')
  })
})
```

## Performance Considerations

- Use `vi.mock()` instead of manual mocking for better performance
- Cleanup state between tests with `afterEach(cleanup)`
- Mock heavy dependencies (file processing, network calls)
- Use `waitFor` for async operations instead of arbitrary timeouts

## Debugging Tests

```bash
# Run specific test file
npm test UploadManager

# Debug mode with Node inspector
npm test -- --inspect-brk

# Verbose output
npm test -- --reporter=verbose

# Coverage for specific files
npm run test:coverage -- --coverage.include="**/UploadManager*"
```

## CI/CD Integration

Tests run automatically on:
- Push to `master`/`main`
- Pull requests
- Pre-commit hooks (optional)

Coverage reports uploaded to Codecov for tracking over time.