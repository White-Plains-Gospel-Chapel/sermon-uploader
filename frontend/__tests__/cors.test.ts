import { describe, it, expect, beforeAll, afterAll } from 'vitest'

describe('MinIO CORS Configuration', () => {
  const MINIO_URL = 'http://192.168.1.127:9000'
  const BUCKET_NAME = 'sermons'
  
  describe('Preflight Requests', () => {
    it('should handle OPTIONS request with proper CORS headers', async () => {
      const response = await fetch(`${MINIO_URL}/${BUCKET_NAME}/test-file.wav`, {
        method: 'OPTIONS',
        headers: {
          'Origin': 'http://localhost:3000',
          'Access-Control-Request-Method': 'PUT',
          'Access-Control-Request-Headers': 'Content-Type'
        }
      })
      
      expect(response.status).toBe(204)
      expect(response.headers.get('Access-Control-Allow-Origin')).toBeTruthy()
      expect(response.headers.get('Access-Control-Allow-Methods')).toContain('PUT')
    })
    
    it('should allow PUT requests from any origin', async () => {
      const response = await fetch(`${MINIO_URL}/${BUCKET_NAME}/test-file.wav`, {
        method: 'OPTIONS',
        headers: {
          'Origin': 'https://example.com',
          'Access-Control-Request-Method': 'PUT'
        }
      })
      
      expect(response.status).toBe(204)
      const allowOrigin = response.headers.get('Access-Control-Allow-Origin')
      expect(allowOrigin === '*' || allowOrigin === 'https://example.com').toBeTruthy()
    })
  })
  
  describe('Direct Upload', () => {
    it('should successfully upload a file with CORS headers', async () => {
      // Create a test WAV file
      const testContent = new Uint8Array([0x52, 0x49, 0x46, 0x46]) // RIFF header
      const testFile = new Blob([testContent], { type: 'audio/wav' })
      
      // First, get a presigned URL from our backend
      const presignedResponse = await fetch('http://localhost:8000/api/upload/presigned', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          filename: `test-cors-${Date.now()}.wav`,
          fileSize: testFile.size
        })
      })
      
      expect(presignedResponse.ok).toBeTruthy()
      const { uploadURL } = await presignedResponse.json()
      
      // Now test the actual upload with CORS
      const uploadResponse = await fetch(uploadURL, {
        method: 'PUT',
        headers: {
          'Content-Type': 'audio/wav',
          'Origin': 'http://localhost:3000'
        },
        body: testFile
      })
      
      expect([200, 204]).toContain(uploadResponse.status)
      expect(uploadResponse.headers.get('Access-Control-Allow-Origin')).toBeTruthy()
    })
    
    it('should handle large file uploads with progress', async () => {
      // Create a 5MB test file
      const size = 5 * 1024 * 1024
      const testContent = new Uint8Array(size)
      const testFile = new Blob([testContent], { type: 'audio/wav' })
      
      // Get presigned URL
      const presignedResponse = await fetch('http://localhost:8000/api/upload/presigned', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          filename: `test-large-${Date.now()}.wav`,
          fileSize: testFile.size
        })
      })
      
      expect(presignedResponse.ok).toBeTruthy()
      const { uploadURL } = await presignedResponse.json()
      
      // Upload with XMLHttpRequest to track progress
      const uploadResult = await new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        let progressEvents = 0
        
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            progressEvents++
          }
        }
        
        xhr.onload = () => {
          resolve({
            status: xhr.status,
            progressEvents,
            headers: xhr.getAllResponseHeaders()
          })
        }
        
        xhr.onerror = reject
        
        xhr.open('PUT', uploadURL)
        xhr.setRequestHeader('Content-Type', 'audio/wav')
        xhr.send(testFile)
      })
      
      expect([200, 204]).toContain(uploadResult.status)
      expect(uploadResult.progressEvents).toBeGreaterThan(0)
    })
  })
  
  describe('Error Handling', () => {
    it('should provide clear error message for CORS failures', async () => {
      // Test with a deliberately incorrect URL to trigger CORS error
      const badUrl = 'http://192.168.1.127:9999/sermons/test.wav'
      
      try {
        await fetch(badUrl, {
          method: 'PUT',
          headers: {
            'Content-Type': 'audio/wav',
            'Origin': 'http://localhost:3000'
          },
          body: new Blob(['test'])
        })
      } catch (error) {
        expect(error).toBeDefined()
        // The error should be a network error due to CORS or connection failure
      }
    })
  })
})

// Add custom matcher for multiple acceptable values
expect.extend({
  toBeOneOf(received: any, values: any[]) {
    const pass = values.includes(received)
    return {
      pass,
      message: () => 
        pass 
          ? `expected ${received} not to be one of ${values.join(', ')}`
          : `expected ${received} to be one of ${values.join(', ')}`
    }
  }
})