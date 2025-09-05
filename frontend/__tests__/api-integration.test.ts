/**
 * API Integration Tests
 */

import { api, createWebSocket } from '@/lib/api'

// Mock fetch for testing
global.fetch = jest.fn()

describe('API Integration Tests', () => {
  beforeEach(() => {
    jest.resetAllMocks()
  })

  describe('API Base URL', () => {
    test('uses correct API base URL', () => {
      // Mock window.location for the test
      Object.defineProperty(window, 'location', {
        value: {
          protocol: 'http:',
          host: 'localhost:3000'
        },
        writable: true
      })

      // Test that API calls use the correct base URL
      ;(fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: 'healthy' })
      })

      api.getStatus()
      expect(fetch).toHaveBeenCalledWith('http://localhost:3000/api/status')
    })
  })

  describe('Upload API', () => {
    test('getPresignedURL makes correct API call', async () => {
      const mockResponse = { 
        uploadUrl: 'https://minio.example.com/upload',
        isDuplicate: false 
      }
      
      ;(fetch as jest.Mock).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse
      })

      const result = await api.getPresignedURL('test.wav', 1024)

      expect(fetch).toHaveBeenCalledWith('http://localhost:3000/api/upload/presigned', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ filename: 'test.wav', fileSize: 1024 })
      })
      expect(result).toEqual(mockResponse)
    })

    test('handles duplicate file error', async () => {
      ;(fetch as jest.Mock).mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: async () => ({ isDuplicate: true, message: 'File already exists' })
      })

      try {
        await api.getPresignedURL('duplicate.wav', 1024)
        fail('Should have thrown an error')
      } catch (error: any) {
        expect(error.name).toBe('DuplicateFileError')
        expect(error.isDuplicate).toBe(true)
      }
    })
  })

  describe('WebSocket Connection', () => {
    test('creates WebSocket with correct URL', () => {
      Object.defineProperty(window, 'location', {
        value: {
          protocol: 'http:',
          host: 'localhost:3000'
        },
        writable: true
      })

      // Mock WebSocket
      global.WebSocket = jest.fn().mockImplementation((url) => ({
        url,
        close: jest.fn(),
        send: jest.fn()
      }))

      const ws = createWebSocket()
      expect(WebSocket).toHaveBeenCalledWith('ws://localhost:3000/ws')
    })

    test('uses WSS for HTTPS', () => {
      Object.defineProperty(window, 'location', {
        value: {
          protocol: 'https:',
          host: 'example.com'
        },
        writable: true
      })

      global.WebSocket = jest.fn()
      createWebSocket()
      expect(WebSocket).toHaveBeenCalledWith('wss://example.com/ws')
    })
  })
})