import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useUploadQueueOptimized } from '@/hooks/useUploadQueueOptimized'

// Simple mocks that just return basic values
vi.mock('@/utils/fileHelpers', () => ({
  filterValidFiles: vi.fn((files) => files),
  generateFileId: vi.fn(() => `mock-id-${Math.random()}`)
}))

vi.mock('@/services/uploadService', () => ({
  uploadService: {
    getPresignedURL: vi.fn(() => Promise.resolve({ 
      uploadUrl: 'https://example.com/upload', 
      isDuplicate: false 
    })),
    getPresignedURLsBatch: vi.fn(() => Promise.resolve({ results: {} })),
    uploadToMinIO: vi.fn(() => Promise.resolve()),
    completeUpload: vi.fn(() => Promise.resolve())
  }
}))

vi.mock('@/utils/concurrency', () => ({
  ConcurrencyLimiter: vi.fn(() => ({
    run: vi.fn((fn) => fn()),
    active: 0
  })),
  getOptimalConcurrency: vi.fn(() => 3),
  delay: vi.fn(() => Promise.resolve())
}))

describe('useUploadQueueOptimized', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('initializes with empty state', () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    expect(result.current.files).toEqual([])
    expect(result.current.isProcessing).toBe(false)
    expect(result.current.stats).toEqual({
      total: 0,
      completed: 0,
      failed: 0,
      duplicates: 0,
      uploading: 0
    })
    expect(result.current.performance.progress).toBe(0)
  })

  it('adds files to the queue', async () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    const testFiles = [
      new File(['content'], 'test1.wav', { type: 'audio/wav' }),
      new File(['content'], 'test2.wav', { type: 'audio/wav' })
    ]
    
    await act(async () => {
      result.current.addFiles(testFiles)
    })
    
    expect(result.current.files.length).toBeGreaterThan(0)
    expect(result.current.stats.total).toBeGreaterThan(0)
  })

  it('removes files from the queue', async () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    const testFile = new File(['content'], 'test.wav', { type: 'audio/wav' })
    
    await act(async () => {
      result.current.addFiles([testFile])
    })
    
    const fileId = result.current.files[0]?.id
    expect(fileId).toBeDefined()
    
    if (fileId) {
      await act(async () => {
        result.current.removeFile(fileId)
      })
      
      expect(result.current.files).toHaveLength(0)
    }
  })

  it('calculates stats correctly', async () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    const testFiles = [
      new File(['content1'], 'test1.wav', { type: 'audio/wav' }),
      new File(['content2'], 'test2.wav', { type: 'audio/wav' })
    ]
    
    await act(async () => {
      result.current.addFiles(testFiles)
    })
    
    expect(result.current.stats.total).toBe(2)
    expect(result.current.stats.completed).toBe(0)
    expect(result.current.stats.failed).toBe(0)
  })

  it('clears completed files', async () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    await act(async () => {
      result.current.clearCompleted()
    })
    
    // Should not throw error even with empty queue
    expect(result.current.files).toEqual([])
  })

  it('formats performance data correctly', () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    expect(result.current.performance).toEqual({
      speed: expect.any(String),
      timeRemaining: expect.any(String),
      progress: expect.any(Number),
      concurrency: expect.any(Number)
    })
  })

  it('provides correct return interface', () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    // Check that all expected methods exist
    expect(typeof result.current.addFiles).toBe('function')
    expect(typeof result.current.removeFile).toBe('function')
    expect(typeof result.current.clearCompleted).toBe('function')
    
    // Check that all expected properties exist
    expect(Array.isArray(result.current.files)).toBe(true)
    expect(typeof result.current.isProcessing).toBe('boolean')
    expect(typeof result.current.stats).toBe('object')
    expect(typeof result.current.performance).toBe('object')
  })

  it('handles empty file arrays', async () => {
    const { result } = renderHook(() => useUploadQueueOptimized())
    
    await act(async () => {
      result.current.addFiles([])
    })
    
    expect(result.current.files).toEqual([])
    expect(result.current.stats.total).toBe(0)
  })
})