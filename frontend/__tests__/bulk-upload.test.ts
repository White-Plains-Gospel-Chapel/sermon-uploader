import { describe, it, expect, beforeAll, afterAll, jest } from '@jest/globals'
import { api } from '../lib/api'

// Increase timeout for large file tests
jest.setTimeout(300000) // 5 minutes

describe('Bulk Upload with Production API', () => {
  const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000'
  
  // Helper to create realistic WAV file
  function createWAVFile(sizeMB: number, filename: string): File {
    const sizeBytes = sizeMB * 1024 * 1024
    
    // WAV header (44 bytes)
    const header = new Uint8Array(44)
    // RIFF chunk descriptor
    header[0] = 0x52; header[1] = 0x49; header[2] = 0x46; header[3] = 0x46 // "RIFF"
    const fileSize = sizeBytes - 8
    header[4] = fileSize & 0xff
    header[5] = (fileSize >> 8) & 0xff
    header[6] = (fileSize >> 16) & 0xff
    header[7] = (fileSize >> 24) & 0xff
    header[8] = 0x57; header[9] = 0x41; header[10] = 0x56; header[11] = 0x45 // "WAVE"
    
    // fmt sub-chunk
    header[12] = 0x66; header[13] = 0x6d; header[14] = 0x74; header[15] = 0x20 // "fmt "
    header[16] = 16; header[17] = 0; header[18] = 0; header[19] = 0 // Subchunk1Size
    header[20] = 1; header[21] = 0 // AudioFormat (PCM)
    header[22] = 2; header[23] = 0 // NumChannels (stereo)
    header[24] = 0x44; header[25] = 0xac; header[26] = 0; header[27] = 0 // SampleRate (44100)
    header[28] = 0x10; header[29] = 0xb1; header[30] = 0x02; header[31] = 0 // ByteRate
    header[32] = 4; header[33] = 0 // BlockAlign
    header[34] = 16; header[35] = 0 // BitsPerSample
    
    // data sub-chunk
    header[36] = 0x64; header[37] = 0x61; header[38] = 0x74; header[39] = 0x61 // "data"
    const dataSize = sizeBytes - 44
    header[40] = dataSize & 0xff
    header[41] = (dataSize >> 8) & 0xff
    header[42] = (dataSize >> 16) & 0xff
    header[43] = (dataSize >> 24) & 0xff
    
    // Create audio data (silence)
    const audioData = new Uint8Array(dataSize)
    
    // Combine header and data
    const fileContent = new Uint8Array(sizeBytes)
    fileContent.set(header, 0)
    fileContent.set(audioData, 44)
    
    return new File([fileContent], filename, { type: 'audio/wav' })
  }
  
  describe('Production Bulk Upload Tests', () => {
    it('should handle 3 files of 200MB each concurrently', async () => {
      const files = [
        createWAVFile(200, `sermon-test-1-${Date.now()}.wav`),
        createWAVFile(200, `sermon-test-2-${Date.now()}.wav`),
        createWAVFile(200, `sermon-test-3-${Date.now()}.wav`)
      ]
      
      console.log('📦 Starting bulk upload test: 3 x 200MB files')
      
      // Get presigned URLs for all files
      const presignedRequests = files.map(file => ({
        filename: file.name,
        fileSize: file.size
      }))
      
      const startTime = Date.now()
      const presignedResponse = await api.getPresignedURLsBatch(presignedRequests)
      
      expect(presignedResponse.urls).toHaveLength(3)
      console.log(`✅ Got presigned URLs in ${Date.now() - startTime}ms`)
      
      // Upload all files concurrently
      const uploadPromises = files.map((file, index) => {
        const uploadInfo = presignedResponse.urls[index]
        let lastProgress = 0
        
        return api.uploadToMinIO(file, uploadInfo.uploadURL, (progress) => {
          if (progress - lastProgress > 10) {
            console.log(`📤 ${file.name}: ${progress.toFixed(1)}%`)
            lastProgress = progress
          }
        })
      })
      
      const uploadStartTime = Date.now()
      await Promise.all(uploadPromises)
      const uploadDuration = (Date.now() - uploadStartTime) / 1000
      
      console.log(`✅ All uploads completed in ${uploadDuration.toFixed(1)}s`)
      expect(uploadDuration).toBeLessThan(120) // Should complete within 2 minutes
      
      // Complete the batch
      const filenames = files.map(f => f.name)
      const completeResponse = await api.completeUploadBatch(filenames)
      
      expect(completeResponse.success).toBeTruthy()
      expect(completeResponse.processed).toBe(3)
    })
    
    it('should handle mixed file sizes efficiently', async () => {
      const files = [
        createWAVFile(50, `sermon-small-${Date.now()}.wav`),
        createWAVFile(150, `sermon-medium-${Date.now()}.wav`),
        createWAVFile(300, `sermon-large-${Date.now()}.wav`)
      ]
      
      console.log('📦 Starting mixed size test: 50MB + 150MB + 300MB')
      
      // Track individual file upload times
      const uploadTimes: Record<string, number> = {}
      
      // Get presigned URLs
      const presignedRequests = files.map(file => ({
        filename: file.name,
        fileSize: file.size
      }))
      
      const presignedResponse = await api.getPresignedURLsBatch(presignedRequests)
      expect(presignedResponse.urls).toHaveLength(3)
      
      // Upload with progress tracking
      const uploadPromises = files.map(async (file, index) => {
        const uploadInfo = presignedResponse.urls[index]
        const startTime = Date.now()
        
        await api.uploadToMinIO(file, uploadInfo.uploadURL, (progress) => {
          if (progress === 100) {
            uploadTimes[file.name] = (Date.now() - startTime) / 1000
          }
        })
      })
      
      await Promise.all(uploadPromises)
      
      // Verify upload times are reasonable
      Object.entries(uploadTimes).forEach(([filename, duration]) => {
        const sizeMB = files.find(f => f.name === filename)!.size / (1024 * 1024)
        const throughputMBps = sizeMB / duration
        console.log(`📊 ${filename}: ${duration.toFixed(1)}s (${throughputMBps.toFixed(1)} MB/s)`)
        
        // Expect at least 1 MB/s throughput
        expect(throughputMBps).toBeGreaterThan(1)
      })
    })
    
    it('should handle CORS correctly for all concurrent uploads', async () => {
      const files = [
        createWAVFile(100, `sermon-cors-1-${Date.now()}.wav`),
        createWAVFile(100, `sermon-cors-2-${Date.now()}.wav`)
      ]
      
      console.log('🔒 Testing CORS with concurrent uploads')
      
      // Get presigned URLs
      const presignedRequests = files.map(file => ({
        filename: file.name,
        fileSize: file.size
      }))
      
      const presignedResponse = await api.getPresignedURLsBatch(presignedRequests)
      
      // Manually upload with XMLHttpRequest to verify CORS headers
      const corsResults = await Promise.all(
        files.map((file, index) => new Promise<any>((resolve) => {
          const xhr = new XMLHttpRequest()
          const uploadURL = presignedResponse.urls[index].uploadURL
          
          xhr.onload = () => {
            resolve({
              status: xhr.status,
              corsAllowed: xhr.status === 200 || xhr.status === 204,
              responseHeaders: xhr.getAllResponseHeaders()
            })
          }
          
          xhr.onerror = () => {
            resolve({
              status: 0,
              corsAllowed: false,
              error: 'Network error - likely CORS issue'
            })
          }
          
          xhr.open('PUT', uploadURL)
          xhr.setRequestHeader('Content-Type', 'audio/wav')
          xhr.setRequestHeader('Origin', window.location.origin)
          xhr.send(file)
        }))
      )
      
      // All uploads should succeed with proper CORS
      corsResults.forEach((result, index) => {
        console.log(`CORS result for file ${index + 1}:`, result)
        expect(result.corsAllowed).toBeTruthy()
        expect(result.status).toBeOneOf([200, 204])
      })
    })
    
    it('should recover from network interruptions during bulk upload', async () => {
      const files = [
        createWAVFile(100, `sermon-retry-1-${Date.now()}.wav`),
        createWAVFile(100, `sermon-retry-2-${Date.now()}.wav`)
      ]
      
      console.log('🔄 Testing network resilience')
      
      // Implement retry logic
      const uploadWithRetry = async (file: File, maxRetries = 3) => {
        let lastError: Error | null = null
        
        for (let attempt = 1; attempt <= maxRetries; attempt++) {
          try {
            // Get fresh presigned URL for each attempt
            const presignedResponse = await api.getPresignedURL(file.name, file.size)
            
            await api.uploadToMinIO(file, presignedResponse.uploadURL, (progress) => {
              console.log(`Attempt ${attempt} - ${file.name}: ${progress.toFixed(1)}%`)
            })
            
            return { success: true, attempts: attempt }
          } catch (error) {
            lastError = error as Error
            console.log(`❌ Attempt ${attempt} failed: ${lastError.message}`)
            
            if (attempt < maxRetries) {
              // Wait before retry with exponential backoff
              const waitTime = Math.min(1000 * Math.pow(2, attempt - 1), 10000)
              console.log(`⏳ Waiting ${waitTime}ms before retry...`)
              await new Promise(resolve => setTimeout(resolve, waitTime))
            }
          }
        }
        
        throw lastError
      }
      
      // Upload with retry logic
      const results = await Promise.all(
        files.map(file => uploadWithRetry(file))
      )
      
      results.forEach(result => {
        expect(result.success).toBeTruthy()
        console.log(`✅ Upload succeeded after ${result.attempts} attempt(s)`)
      })
    })
  })
  
  describe('Performance Benchmarks', () => {
    it('should maintain good throughput with 500MB+ total upload', async () => {
      const files = [
        createWAVFile(250, `sermon-perf-1-${Date.now()}.wav`),
        createWAVFile(250, `sermon-perf-2-${Date.now()}.wav`)
      ]
      
      console.log('⚡ Performance test: 500MB total upload')
      
      const totalSize = files.reduce((sum, f) => sum + f.size, 0)
      const totalSizeMB = totalSize / (1024 * 1024)
      
      // Get presigned URLs
      const presignedRequests = files.map(file => ({
        filename: file.name,
        fileSize: file.size
      }))
      
      const presignedResponse = await api.getPresignedURLsBatch(presignedRequests)
      
      // Upload and measure throughput
      const startTime = Date.now()
      let totalUploaded = 0
      
      await Promise.all(
        files.map((file, index) => 
          api.uploadToMinIO(file, presignedResponse.urls[index].uploadURL, (progress) => {
            const uploaded = (file.size * progress) / 100
            totalUploaded += uploaded
            const elapsedSeconds = (Date.now() - startTime) / 1000
            const throughputMBps = (totalUploaded / (1024 * 1024)) / elapsedSeconds
            
            if (progress % 25 === 0) {
              console.log(`📈 Overall throughput: ${throughputMBps.toFixed(2)} MB/s`)
            }
          })
        )
      )
      
      const totalDuration = (Date.now() - startTime) / 1000
      const avgThroughput = totalSizeMB / totalDuration
      
      console.log(`✅ Completed ${totalSizeMB}MB in ${totalDuration.toFixed(1)}s`)
      console.log(`📊 Average throughput: ${avgThroughput.toFixed(2)} MB/s`)
      
      // Expect at least 2 MB/s average throughput for good performance
      expect(avgThroughput).toBeGreaterThan(2)
    })
  })
})

// Add custom matcher
expect.extend({
  toBeOneOf(received, values) {
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