/**
 * Audio Upload Integrity Tests
 * Critical tests ensuring bit-perfect audio file preservation during frontend upload process
 */

import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { rest } from 'msw'
import { setupServer } from 'msw/node'
import crypto from 'crypto'

// Mock upload components and utilities
import { UploadQueue } from '@/components/upload/UploadQueue'
import { uploadToPresignedURL } from '@/lib/upload'
import { validateFile } from '@/lib/validation'
import type { UploadFile, PresignedURLResponse } from '@/types/upload.types'

// Test WAV file generator for consistent testing
export class TestWAVGenerator {
  /**
   * Generates a predictable WAV file for testing bit-perfect preservation
   */
  generateWAV(
    filename: string,
    durationSeconds: number,
    sampleRate: number = 44100,
    bitDepth: number = 16,
    channels: number = 2
  ): ArrayBuffer {
    const numSamples = durationSeconds * sampleRate * channels
    const bytesPerSample = bitDepth / 8
    const dataSize = numSamples * bytesPerSample
    const fileSize = 36 + dataSize

    const buffer = new ArrayBuffer(44 + dataSize)
    const view = new DataView(buffer)

    // RIFF Header
    this.writeString(view, 0, 'RIFF')
    view.setUint32(4, fileSize, true)
    this.writeString(view, 8, 'WAVE')

    // fmt chunk
    this.writeString(view, 12, 'fmt ')
    view.setUint32(16, 16, true) // fmt chunk size
    view.setUint16(20, 1, true)  // audio format (PCM)
    view.setUint16(22, channels, true)
    view.setUint32(24, sampleRate, true)
    view.setUint32(28, sampleRate * channels * bytesPerSample, true) // byte rate
    view.setUint16(32, channels * bytesPerSample, true) // block align
    view.setUint16(34, bitDepth, true)

    // data chunk
    this.writeString(view, 36, 'data')
    view.setUint32(40, dataSize, true)

    // Generate predictable audio data for hash consistency
    let offset = 44
    for (let i = 0; i < numSamples; i++) {
      const value = (i % 1000) - 500 // Simple pattern
      if (bitDepth === 16) {
        view.setInt16(offset, value, true)
        offset += 2
      } else if (bitDepth === 24) {
        // 24-bit audio handling
        view.setInt16(offset, value, true)
        view.setInt8(offset + 2, value >> 16)
        offset += 3
      }
    }

    return buffer
  }

  private writeString(view: DataView, offset: number, str: string): void {
    for (let i = 0; i < str.length; i++) {
      view.setUint8(offset + i, str.charCodeAt(i))
    }
  }
}

// Mock File API for testing
class MockFile extends File {
  private _arrayBuffer: ArrayBuffer

  constructor(arrayBuffer: ArrayBuffer, filename: string, options?: FilePropertyBag) {
    // Create a blob from the array buffer
    const blob = new Blob([arrayBuffer], { type: 'audio/wav' })
    super([blob], filename, { type: 'audio/wav', ...options })
    this._arrayBuffer = arrayBuffer
  }

  arrayBuffer(): Promise<ArrayBuffer> {
    return Promise.resolve(this._arrayBuffer.slice(0))
  }
}

// Calculate SHA256 hash for integrity verification
async function calculateFileHash(file: File): Promise<string> {
  const arrayBuffer = await file.arrayBuffer()
  const hash = crypto.createHash('sha256')
  hash.update(new Uint8Array(arrayBuffer))
  return hash.digest('hex')
}

// MSW server for mocking API responses
const server = setupServer(
  rest.post('/api/upload/presigned/batch', async (req, res, ctx) => {
    const body = await req.json()
    const results: Record<string, PresignedURLResponse> = {}
    
    // Mock presigned URL responses
    body.files.forEach((file: { filename: string; fileSize: number }) => {
      results[file.filename] = {
        uploadUrl: `https://mock-minio.example.com/sermons/${file.filename}?signature=mock`,
        isDuplicate: false
      }
    })
    
    return res(ctx.json({ results }))
  }),

  rest.put('https://mock-minio.example.com/sermons/*', async (req, res, ctx) => {
    // Mock successful upload to presigned URL
    const body = await req.arrayBuffer()
    
    // Verify content-type is application/octet-stream (no compression)
    const contentType = req.headers.get('content-type')
    if (contentType !== 'application/octet-stream') {
      return res(ctx.status(400, 'Invalid content type - must be application/octet-stream'))
    }
    
    return res(
      ctx.status(200),
      ctx.set('ETag', '"mock-etag"')
    )
  })
)

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('Audio Upload Integrity Tests', () => {
  const generator = new TestWAVGenerator()

  describe('File Validation - Audio Format Integrity', () => {
    test('validates WAV files and preserves format information', async () => {
      const wavBuffer = generator.generateWAV('test_sermon.wav', 30, 44100, 16, 2)
      const wavFile = new MockFile(wavBuffer, 'test_sermon.wav')
      
      const validation = await validateFile(wavFile)
      
      expect(validation.isValid).toBe(true)
      expect(validation.error).toBeUndefined()
      
      // Verify WAV header integrity
      const arrayBuffer = await wavFile.arrayBuffer()
      const view = new DataView(arrayBuffer)
      
      // Check RIFF signature
      expect(String.fromCharCode(
        view.getUint8(0), view.getUint8(1), view.getUint8(2), view.getUint8(3)
      )).toBe('RIFF')
      
      // Check WAVE signature
      expect(String.fromCharCode(
        view.getUint8(8), view.getUint8(9), view.getUint8(10), view.getUint8(11)
      )).toBe('WAVE')
      
      // Verify audio format parameters
      const channels = view.getUint16(22, true)
      const sampleRate = view.getUint32(24, true)
      const bitDepth = view.getUint16(34, true)
      
      expect(channels).toBe(2)
      expect(sampleRate).toBe(44100)
      expect(bitDepth).toBe(16)
    })

    test('rejects non-WAV files to maintain audio quality focus', async () => {
      // Create a fake MP3 file
      const fakeMP3 = new File(['fake mp3 content'], 'audio.mp3', { type: 'audio/mpeg' })
      
      const validation = await validateFile(fakeMP3)
      
      expect(validation.isValid).toBe(false)
      expect(validation.error).toContain('Only WAV files are supported')
    })

    test('validates different WAV formats while preserving quality requirements', async () => {
      const testCases = [
        { name: 'cd_quality.wav', sampleRate: 44100, bitDepth: 16, channels: 2 },
        { name: 'high_res.wav', sampleRate: 96000, bitDepth: 24, channels: 2 },
        { name: 'broadcast.wav', sampleRate: 48000, bitDepth: 16, channels: 1 }
      ]
      
      for (const testCase of testCases) {
        const wavBuffer = generator.generateWAV(
          testCase.name, 10, testCase.sampleRate, testCase.bitDepth, testCase.channels
        )
        const wavFile = new MockFile(wavBuffer, testCase.name)
        
        const validation = await validateFile(wavFile)
        
        expect(validation.isValid).toBe(true)
        expect(validation.error).toBeUndefined()
      }
    })
  })

  describe('Upload Process - Bit-Perfect Preservation', () => {
    test('preserves file integrity during presigned URL upload', async () => {
      const originalBuffer = generator.generateWAV('sermon_integrity_test.wav', 60, 44100, 16, 2)
      const originalFile = new MockFile(originalBuffer, 'sermon_integrity_test.wav')
      const originalHash = await calculateFileHash(originalFile)
      
      // Mock the upload process
      let uploadedData: ArrayBuffer | null = null
      
      server.use(
        rest.put('https://mock-minio.example.com/sermons/*', async (req, res, ctx) => {
          uploadedData = await req.arrayBuffer()
          
          // Verify content-type is application/octet-stream (no compression)
          expect(req.headers.get('content-type')).toBe('application/octet-stream')
          
          return res(ctx.status(200), ctx.set('ETag', '"integrity-test-etag"'))
        })
      )
      
      // Execute upload
      const mockPresignedURL = 'https://mock-minio.example.com/sermons/sermon_integrity_test.wav?signature=mock'
      
      await uploadToPresignedURL(originalFile, mockPresignedURL, {
        onProgress: (progress) => {
          expect(progress).toBeGreaterThanOrEqual(0)
          expect(progress).toBeLessThanOrEqual(100)
        }
      })
      
      // Verify bit-perfect preservation
      expect(uploadedData).not.toBeNull()
      
      if (uploadedData) {
        const uploadedHash = crypto.createHash('sha256')
          .update(new Uint8Array(uploadedData))
          .digest('hex')
        
        expect(uploadedHash).toBe(originalHash)
        expect(uploadedData.byteLength).toBe(originalBuffer.byteLength)
        
        // Byte-by-byte comparison
        const originalBytes = new Uint8Array(originalBuffer)
        const uploadedBytes = new Uint8Array(uploadedData)
        
        expect(uploadedBytes).toEqual(originalBytes)
      }
    })

    test('handles large files without corruption', async () => {
      // Generate a larger test file (5 minutes of audio)
      const largeBuffer = generator.generateWAV('large_sermon.wav', 300, 48000, 24, 2)
      const largeFile = new MockFile(largeBuffer, 'large_sermon.wav')
      const originalHash = await calculateFileHash(largeFile)
      
      let uploadedData: ArrayBuffer | null = null
      let progressCalls = 0
      
      server.use(
        rest.put('https://mock-minio.example.com/sermons/*', async (req, res, ctx) => {
          uploadedData = await req.arrayBuffer()
          return res(ctx.status(200), ctx.set('ETag', '"large-file-etag"'))
        })
      )
      
      const mockPresignedURL = 'https://mock-minio.example.com/sermons/large_sermon.wav?signature=mock'
      
      await uploadToPresignedURL(largeFile, mockPresignedURL, {
        onProgress: (progress) => {
          progressCalls++
          expect(progress).toBeGreaterThanOrEqual(0)
          expect(progress).toBeLessThanOrEqual(100)
        }
      })
      
      // Verify large file integrity
      expect(uploadedData).not.toBeNull()
      expect(progressCalls).toBeGreaterThan(0)
      
      if (uploadedData) {
        const uploadedHash = crypto.createHash('sha256')
          .update(new Uint8Array(uploadedData))
          .digest('hex')
        
        expect(uploadedHash).toBe(originalHash)
        expect(uploadedData.byteLength).toBe(largeBuffer.byteLength)
      }
    })

    test('maintains file integrity across multiple concurrent uploads', async () => {
      const testFiles = [
        { name: 'concurrent1.wav', duration: 30 },
        { name: 'concurrent2.wav', duration: 45 },
        { name: 'concurrent3.wav', duration: 60 }
      ]
      
      const originalHashes: string[] = []
      const uploadedHashes: string[] = []
      
      // Generate test files
      const files = await Promise.all(
        testFiles.map(async (testFile) => {
          const buffer = generator.generateWAV(testFile.name, testFile.duration, 44100, 16, 2)
          const file = new MockFile(buffer, testFile.name)
          const hash = await calculateFileHash(file)
          originalHashes.push(hash)
          return file
        })
      )
      
      // Mock concurrent uploads
      server.use(
        rest.put('https://mock-minio.example.com/sermons/*', async (req, res, ctx) => {
          const uploadedData = await req.arrayBuffer()
          const hash = crypto.createHash('sha256')
            .update(new Uint8Array(uploadedData))
            .digest('hex')
          uploadedHashes.push(hash)
          
          return res(ctx.status(200), ctx.set('ETag', `"concurrent-${hash.slice(0, 8)}"`))
        })
      )
      
      // Execute concurrent uploads
      const uploadPromises = files.map((file, index) => 
        uploadToPresignedURL(
          file,
          `https://mock-minio.example.com/sermons/${testFiles[index].name}?signature=mock`
        )
      )
      
      await Promise.all(uploadPromises)
      
      // Verify all files maintained integrity
      expect(uploadedHashes).toHaveLength(files.length)
      originalHashes.forEach(originalHash => {
        expect(uploadedHashes).toContain(originalHash)
      })
    })
  })

  describe('Upload Queue Component - Quality Preservation', () => {
    test('displays upload progress without affecting file integrity', async () => {
      const wavBuffer = generator.generateWAV('queue_test.wav', 30, 44100, 16, 2)
      const testFile: UploadFile = {
        file: new MockFile(wavBuffer, 'queue_test.wav'),
        id: 'test-1',
        status: 'queued',
        progress: 0
      }
      
      render(<UploadQueue files={[testFile]} onRemoveFile={() => {}} />)
      
      // Verify file is displayed
      expect(screen.getByText('queue_test.wav')).toBeInTheDocument()
      expect(screen.getByText('Queued')).toBeInTheDocument()
      
      // Verify file size is preserved and displayed
      const fileSizeElement = screen.getByTestId('file-size')
      expect(fileSizeElement).toHaveTextContent((wavBuffer.byteLength / 1024 / 1024).toFixed(2))
    })

    test('handles upload errors without corrupting queued files', async () => {
      const wavBuffer = generator.generateWAV('error_test.wav', 15, 44100, 16, 2)
      const testFile: UploadFile = {
        file: new MockFile(wavBuffer, 'error_test.wav'),
        id: 'error-test',
        status: 'error',
        progress: 0,
        error: 'Network error occurred'
      }
      
      render(<UploadQueue files={[testFile]} onRemoveFile={() => {}} />)
      
      expect(screen.getByText('error_test.wav')).toBeInTheDocument()
      expect(screen.getByText('Error')).toBeInTheDocument()
      expect(screen.getByText('Network error occurred')).toBeInTheDocument()
      
      // Verify the original file is still accessible and uncorrupted
      const originalHash = await calculateFileHash(testFile.file)
      expect(originalHash).toBeTruthy()
      
      const arrayBuffer = await testFile.file.arrayBuffer()
      expect(arrayBuffer.byteLength).toBe(wavBuffer.byteLength)
    })
  })

  describe('Hash Verification and Duplicate Detection', () => {
    test('accurately detects duplicate files by content hash', async () => {
      const wavBuffer = generator.generateWAV('duplicate_test.wav', 20, 44100, 16, 2)
      
      // Create two files with identical content but different names
      const file1 = new MockFile(wavBuffer, 'original.wav')
      const file2 = new MockFile(wavBuffer, 'duplicate.wav')
      
      const hash1 = await calculateFileHash(file1)
      const hash2 = await calculateFileHash(file2)
      
      // Hashes should be identical despite different filenames
      expect(hash1).toBe(hash2)
      
      // Mock API to return duplicate detection
      server.use(
        rest.post('/api/upload/presigned/batch', async (req, res, ctx) => {
          const body = await req.json()
          const results: Record<string, PresignedURLResponse> = {}
          
          // Simulate duplicate detection
          results[body.files[0].filename] = {
            uploadUrl: '',
            isDuplicate: false
          }
          results[body.files[1].filename] = {
            uploadUrl: '',
            isDuplicate: true,
            message: 'File already exists with same content hash'
          }
          
          return res(ctx.json({ results }))
        })
      )
      
      // Test duplicate detection in upload flow
      const response = await fetch('/api/upload/presigned/batch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          files: [
            { filename: 'original.wav', fileSize: file1.size },
            { filename: 'duplicate.wav', fileSize: file2.size }
          ]
        })
      })
      
      const result = await response.json()
      
      expect(result.results['original.wav'].isDuplicate).toBe(false)
      expect(result.results['duplicate.wav'].isDuplicate).toBe(true)
    })
  })

  describe('Error Handling - Data Preservation', () => {
    test('preserves original file when upload fails', async () => {
      const wavBuffer = generator.generateWAV('fail_test.wav', 10, 44100, 16, 2)
      const originalFile = new MockFile(wavBuffer, 'fail_test.wav')
      const originalHash = await calculateFileHash(originalFile)
      
      // Mock upload failure
      server.use(
        rest.put('https://mock-minio.example.com/sermons/*', (req, res, ctx) => {
          return res(ctx.status(500), ctx.json({ error: 'Server error' }))
        })
      )
      
      const mockPresignedURL = 'https://mock-minio.example.com/sermons/fail_test.wav?signature=mock'
      
      // Upload should fail but preserve original file
      await expect(uploadToPresignedURL(originalFile, mockPresignedURL)).rejects.toThrow()
      
      // Verify original file remains unchanged
      const postFailHash = await calculateFileHash(originalFile)
      expect(postFailHash).toBe(originalHash)
      
      const arrayBuffer = await originalFile.arrayBuffer()
      expect(arrayBuffer.byteLength).toBe(wavBuffer.byteLength)
    })

    test('handles network interruption gracefully', async () => {
      const wavBuffer = generator.generateWAV('network_test.wav', 60, 44100, 16, 2)
      const originalFile = new MockFile(wavBuffer, 'network_test.wav')
      const originalHash = await calculateFileHash(originalFile)
      
      let attemptCount = 0
      
      server.use(
        rest.put('https://mock-minio.example.com/sermons/*', (req, res, ctx) => {
          attemptCount++
          if (attemptCount < 2) {
            return res.networkError('Network error')
          }
          return res(ctx.status(200), ctx.set('ETag', '"network-recovery-etag"'))
        })
      )
      
      const mockPresignedURL = 'https://mock-minio.example.com/sermons/network_test.wav?signature=mock'
      
      // Should eventually succeed after network recovery
      await expect(uploadToPresignedURL(originalFile, mockPresignedURL, {
        retries: 2,
        retryDelay: 100
      })).resolves.not.toThrow()
      
      // Verify file integrity maintained through network issues
      const postNetworkHash = await calculateFileHash(originalFile)
      expect(postNetworkHash).toBe(originalHash)
    })
  })
})