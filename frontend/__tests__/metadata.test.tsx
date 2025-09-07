import { describe, it, expect, vi } from 'vitest'

// Mock the Inter font import to avoid font loading issues in tests
vi.mock('next/font/google', () => ({
  Inter: vi.fn(() => ({
    className: 'mocked-inter-font'
  }))
}))

describe('Metadata Configuration', () => {
  it('should export viewport separately from metadata', async () => {
    // This test should FAIL with current code where viewport is in metadata
    const layoutModule = await import('@/app/layout')
    
    // Viewport should be exported separately for Next.js 14
    expect(layoutModule.viewport).toBeDefined()
    
    // Metadata should not contain viewport property
    expect(layoutModule.metadata.viewport).toBeUndefined()
  })

  it('should have correct viewport configuration', async () => {
    const { viewport } = await import('@/app/layout')
    
    expect(viewport).toEqual({
      width: 'device-width',
      initialScale: 1,
      maximumScale: 1,
    })
  })

  it('should have proper metadata structure without viewport', async () => {
    const { metadata } = await import('@/app/layout')
    
    expect(metadata).toEqual({
      title: 'Sermon Uploader',
      description: 'Upload and manage sermon audio files with automatic processing',
      keywords: ['sermon', 'upload', 'audio', 'church', 'management'],
      authors: [{ name: 'Sermon Uploader Team' }],
    })
    
    // Ensure viewport is not present in metadata
    expect(metadata).not.toHaveProperty('viewport')
  })
})