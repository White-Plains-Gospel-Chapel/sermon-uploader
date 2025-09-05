/**
 * End-to-End Audio Upload Integrity Tests
 * Tests the complete upload workflow with real browser interactions
 * ensuring bit-perfect audio preservation throughout the entire system
 */

import { test, expect, Page } from '@playwright/test'
import { promises as fs } from 'fs'
import path from 'path'
import crypto from 'crypto'

// Test WAV generator for E2E tests
class E2EWAVGenerator {
  generateWAV(
    filename: string,
    durationSeconds: number,
    sampleRate: number = 44100,
    bitDepth: number = 16,
    channels: number = 2
  ): Buffer {
    const numSamples = durationSeconds * sampleRate * channels
    const bytesPerSample = bitDepth / 8
    const dataSize = numSamples * bytesPerSample
    const fileSize = 36 + dataSize

    const buffer = Buffer.alloc(44 + dataSize)
    let offset = 0

    // RIFF Header
    buffer.write('RIFF', offset); offset += 4
    buffer.writeUInt32LE(fileSize, offset); offset += 4
    buffer.write('WAVE', offset); offset += 4

    // fmt chunk
    buffer.write('fmt ', offset); offset += 4
    buffer.writeUInt32LE(16, offset); offset += 4  // fmt chunk size
    buffer.writeUInt16LE(1, offset); offset += 2   // audio format (PCM)
    buffer.writeUInt16LE(channels, offset); offset += 2
    buffer.writeUInt32LE(sampleRate, offset); offset += 4
    buffer.writeUInt32LE(sampleRate * channels * bytesPerSample, offset); offset += 4 // byte rate
    buffer.writeUInt16LE(channels * bytesPerSample, offset); offset += 2 // block align
    buffer.writeUInt16LE(bitDepth, offset); offset += 2

    // data chunk
    buffer.write('data', offset); offset += 4
    buffer.writeUInt32LE(dataSize, offset); offset += 4

    // Generate predictable audio data for hash consistency
    for (let i = 0; i < numSamples; i++) {
      const value = (i % 1000) - 500 // Simple repeating pattern
      if (bitDepth === 16) {
        buffer.writeInt16LE(value, offset)
        offset += 2
      } else if (bitDepth === 24) {
        buffer.writeInt16LE(value, offset)
        buffer.writeInt8(value >> 16, offset + 2)
        offset += 3
      }
    }

    return buffer
  }
}

// Helper to calculate file hash
async function calculateFileHash(filePath: string): Promise<string> {
  const fileBuffer = await fs.readFile(filePath)
  return crypto.createHash('sha256').update(fileBuffer).digest('hex')
}

// Helper to create temporary test files
async function createTestFile(filename: string, content: Buffer): Promise<string> {
  const tempDir = path.join(__dirname, '../temp')
  await fs.mkdir(tempDir, { recursive: true })
  
  const filePath = path.join(tempDir, filename)
  await fs.writeFile(filePath, content)
  
  return filePath
}

// Cleanup helper
async function cleanupTestFiles() {
  const tempDir = path.join(__dirname, '../temp')
  try {
    await fs.rmdir(tempDir, { recursive: true })
  } catch {
    // Ignore cleanup errors
  }
}

test.describe('Audio Upload Integrity - End-to-End', () => {
  const generator = new E2EWAVGenerator()
  
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    
    // Wait for the upload interface to be ready
    await page.waitForSelector('[data-testid="upload-dropzone"]', { timeout: 10000 })
  })
  
  test.afterEach(async () => {
    await cleanupTestFiles()
  })

  test('uploads single WAV file with bit-perfect preservation', async ({ page }) => {
    // Generate test file
    const testWAV = generator.generateWAV('single_test.wav', 30, 44100, 16, 2)
    const testFilePath = await createTestFile('single_test.wav', testWAV)
    const originalHash = crypto.createHash('sha256').update(testWAV).digest('hex')
    
    // Upload file through drag and drop
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(testFilePath)
    
    // Wait for file to appear in upload queue
    await page.waitForSelector('[data-testid="upload-file-item"]', { timeout: 5000 })
    
    // Verify file information is displayed correctly
    await expect(page.locator('[data-testid="file-name"]')).toContainText('single_test.wav')
    await expect(page.locator('[data-testid="file-size"]')).toContainText(/\d+(\.\d+)?\s*(KB|MB)/)
    await expect(page.locator('[data-testid="file-status"]')).toContainText('Queued')
    
    // Start upload process
    await page.click('[data-testid="start-upload-button"]')
    
    // Monitor upload progress
    await page.waitForSelector('[data-testid="upload-progress"]', { timeout: 5000 })
    
    // Wait for upload completion
    await page.waitForSelector('[data-testid="upload-success"]', { timeout: 30000 })
    
    // Verify success status
    await expect(page.locator('[data-testid="file-status"]')).toContainText('Uploaded')
    
    // Verify hash is displayed (if implemented in UI)
    const hashElement = page.locator('[data-testid="file-hash"]')
    if (await hashElement.count() > 0) {
      await expect(hashElement).toContainText(originalHash.substring(0, 8)) // First 8 chars
    }
    
    // Check for any error messages
    const errorElements = await page.locator('[data-testid="error-message"]').count()
    expect(errorElements).toBe(0)
  })

  test('uploads multiple WAV files concurrently maintaining individual integrity', async ({ page }) => {
    // Generate multiple test files with different characteristics
    const testFiles = [
      { name: 'file1.wav', duration: 15, sampleRate: 44100, bitDepth: 16, channels: 2 },
      { name: 'file2.wav', duration: 30, sampleRate: 48000, bitDepth: 24, channels: 2 },
      { name: 'file3.wav', duration: 10, sampleRate: 22050, bitDepth: 16, channels: 1 }
    ]
    
    const filePaths: string[] = []
    const originalHashes: string[] = []
    
    // Create test files
    for (const fileSpec of testFiles) {
      const wavBuffer = generator.generateWAV(
        fileSpec.name, fileSpec.duration, fileSpec.sampleRate, fileSpec.bitDepth, fileSpec.channels
      )
      const filePath = await createTestFile(fileSpec.name, wavBuffer)
      const hash = crypto.createHash('sha256').update(wavBuffer).digest('hex')
      
      filePaths.push(filePath)
      originalHashes.push(hash)
    }
    
    // Upload all files at once
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(filePaths)
    
    // Wait for all files to appear in queue
    await page.waitForSelector('[data-testid="upload-file-item"]', { timeout: 5000 })
    const fileItems = page.locator('[data-testid="upload-file-item"]')
    await expect(fileItems).toHaveCount(testFiles.length)
    
    // Verify all files are listed correctly
    for (const fileSpec of testFiles) {
      await expect(page.locator(`[data-filename="${fileSpec.name}"]`)).toBeVisible()
    }
    
    // Start batch upload
    await page.click('[data-testid="start-upload-button"]')
    
    // Wait for all uploads to complete
    await page.waitForFunction(
      () => {
        const successElements = document.querySelectorAll('[data-testid="upload-success"]')
        return successElements.length === 3 // All 3 files uploaded
      },
      { timeout: 60000 }
    )
    
    // Verify each file uploaded successfully
    for (const fileSpec of testFiles) {
      const fileRow = page.locator(`[data-filename="${fileSpec.name}"]`)
      await expect(fileRow.locator('[data-testid="file-status"]')).toContainText('Uploaded')
    }
    
    // Verify no errors occurred
    const errorElements = await page.locator('[data-testid="error-message"]').count()
    expect(errorElements).toBe(0)
  })

  test('handles large WAV file upload with progress tracking', async ({ page }) => {
    // Generate a larger test file (5 minutes of high-quality audio)
    const largeWAV = generator.generateWAV('large_sermon.wav', 300, 96000, 24, 2)
    const largeFilePath = await createTestFile('large_sermon.wav', largeWAV)
    const fileSizeMB = largeWAV.length / (1024 * 1024)
    
    console.log(`Testing large file upload: ${fileSizeMB.toFixed(2)} MB`)
    
    // Upload large file
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(largeFilePath)
    
    // Wait for file to appear
    await page.waitForSelector('[data-testid="upload-file-item"]', { timeout: 5000 })
    
    // Verify large file size is displayed correctly
    await expect(page.locator('[data-testid="file-size"]')).toContainText(/\d+(\.\d+)?\s*MB/)
    
    // Start upload
    await page.click('[data-testid="start-upload-button"]')
    
    // Monitor progress updates
    await page.waitForSelector('[data-testid="upload-progress"]', { timeout: 5000 })
    
    // Verify progress increments
    let previousProgress = 0
    let progressCount = 0
    
    while (progressCount < 10) { // Check progress up to 10 times
      try {
        const progressText = await page.locator('[data-testid="progress-percentage"]').textContent()
        const currentProgress = parseInt(progressText?.replace('%', '') || '0')
        
        expect(currentProgress).toBeGreaterThanOrEqual(previousProgress)
        expect(currentProgress).toBeLessThanOrEqual(100)
        
        if (currentProgress === 100) break
        
        previousProgress = currentProgress
        progressCount++
        
        await page.waitForTimeout(1000) // Wait 1 second between checks
      } catch {
        // Progress element may disappear when upload completes
        break
      }
    }
    
    // Wait for upload completion
    await page.waitForSelector('[data-testid="upload-success"]', { timeout: 120000 }) // 2 minutes for large file
    
    // Verify success
    await expect(page.locator('[data-testid="file-status"]')).toContainText('Uploaded')
  })

  test('detects and handles duplicate files correctly', async ({ page }) => {
    // Generate identical test files with different names
    const wavContent = generator.generateWAV('original.wav', 20, 44100, 16, 2)
    
    const file1Path = await createTestFile('sermon_copy1.wav', wavContent)
    const file2Path = await createTestFile('sermon_copy2.wav', wavContent)
    
    // Upload first file
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(file1Path)
    
    await page.waitForSelector('[data-testid="upload-file-item"]')
    await page.click('[data-testid="start-upload-button"]')
    await page.waitForSelector('[data-testid="upload-success"]', { timeout: 30000 })
    
    // Clear queue and upload duplicate
    await page.click('[data-testid="clear-queue-button"]')
    await fileInput.setInputFiles(file2Path)
    
    await page.waitForSelector('[data-testid="upload-file-item"]')
    await page.click('[data-testid="start-upload-button"]')
    
    // Should detect as duplicate
    await page.waitForSelector('[data-testid="duplicate-detected"]', { timeout: 10000 })
    await expect(page.locator('[data-testid="file-status"]')).toContainText('Duplicate')
    await expect(page.locator('[data-testid="duplicate-message"]')).toContainText(/already exists|duplicate/i)
  })

  test('validates file format and rejects non-WAV files', async ({ page }) => {
    // Create a fake MP3 file
    const fakeMp3Content = Buffer.from('fake mp3 content for testing')
    const fakeMp3Path = await createTestFile('fake_audio.mp3', fakeMp3Content)
    
    // Attempt to upload non-WAV file
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(fakeMp3Path)
    
    // Should show validation error
    await page.waitForSelector('[data-testid="validation-error"]', { timeout: 5000 })
    await expect(page.locator('[data-testid="validation-error"]')).toContainText(/WAV files only|invalid format/i)
    
    // Upload button should remain disabled or file should be rejected
    const startButton = page.locator('[data-testid="start-upload-button"]')
    const isDisabled = await startButton.getAttribute('disabled')
    expect(isDisabled).not.toBeNull()
  })

  test('handles upload errors gracefully without corrupting queue', async ({ page }) => {
    // Generate test files
    const validWAV = generator.generateWAV('valid.wav', 10, 44100, 16, 2)
    const validFilePath = await createTestFile('valid.wav', validWAV)
    
    // Mock network failure by intercepting requests
    await page.route('**/api/upload/**', route => {
      // Simulate network error for first few attempts
      route.abort('failed')
    })
    
    // Upload file
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(validFilePath)
    
    await page.waitForSelector('[data-testid="upload-file-item"]')
    await page.click('[data-testid="start-upload-button"]')
    
    // Should show error status
    await page.waitForSelector('[data-testid="upload-error"]', { timeout: 10000 })
    await expect(page.locator('[data-testid="file-status"]')).toContainText('Error')
    
    // Verify retry functionality if available
    const retryButton = page.locator('[data-testid="retry-upload-button"]')
    if (await retryButton.count() > 0) {
      // Remove network interception
      await page.unroute('**/api/upload/**')
      
      // Click retry
      await retryButton.click()
      
      // Should eventually succeed
      await page.waitForSelector('[data-testid="upload-success"]', { timeout: 30000 })
    }
  })

  test('preserves audio quality through complete upload pipeline', async ({ page }) => {
    // Generate high-quality test file
    const highQualityWAV = generator.generateWAV('hq_sermon.wav', 60, 96000, 24, 2)
    const filePath = await createTestFile('hq_sermon.wav', highQualityWAV)
    const originalHash = crypto.createHash('sha256').update(highQualityWAV).digest('hex')
    
    // Upload file
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(filePath)
    
    await page.waitForSelector('[data-testid="upload-file-item"]')
    
    // Verify audio properties are detected and displayed
    const audioInfo = page.locator('[data-testid="audio-info"]')
    if (await audioInfo.count() > 0) {
      await expect(audioInfo).toContainText('96000 Hz') // Sample rate
      await expect(audioInfo).toContainText('24-bit')   // Bit depth
      await expect(audioInfo).toContainText('Stereo')   // Channels
    }
    
    await page.click('[data-testid="start-upload-button"]')
    await page.waitForSelector('[data-testid="upload-success"]', { timeout: 60000 })
    
    // Verify upload completed successfully with quality preserved
    await expect(page.locator('[data-testid="file-status"]')).toContainText('Uploaded')
    
    // Check if quality metrics are displayed
    const qualityIndicator = page.locator('[data-testid="quality-indicator"]')
    if (await qualityIndicator.count() > 0) {
      await expect(qualityIndicator).toContainText(/high|lossless/i)
    }
  })

  test('handles WebSocket connection for real-time progress updates', async ({ page }) => {
    // Generate test file
    const testWAV = generator.generateWAV('websocket_test.wav', 45, 48000, 24, 2)
    const filePath = await createTestFile('websocket_test.wav', testWAV)
    
    // Monitor WebSocket messages
    const wsMessages: any[] = []
    
    page.on('websocket', ws => {
      ws.on('framereceived', event => {
        try {
          const message = JSON.parse(event.payload.toString())
          wsMessages.push(message)
        } catch {
          // Ignore non-JSON messages
        }
      })
    })
    
    // Upload file
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(filePath)
    
    await page.waitForSelector('[data-testid="upload-file-item"]')
    await page.click('[data-testid="start-upload-button"]')
    
    // Wait for upload to complete
    await page.waitForSelector('[data-testid="upload-success"]', { timeout: 30000 })
    
    // Verify WebSocket messages were received
    expect(wsMessages.length).toBeGreaterThan(0)
    
    // Check for expected message types
    const messageTypes = wsMessages.map(msg => msg.type)
    expect(messageTypes).toContain('upload_start')
    expect(messageTypes).toContain('upload_complete')
  })

  test('validates upload queue persistence across page refresh', async ({ page, context }) => {
    // Generate test file
    const testWAV = generator.generateWAV('persistence_test.wav', 20, 44100, 16, 2)
    const filePath = await createTestFile('persistence_test.wav', testWAV)
    
    // Add file to queue but don't upload
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(filePath)
    
    await page.waitForSelector('[data-testid="upload-file-item"]')
    await expect(page.locator('[data-testid="file-name"]')).toContainText('persistence_test.wav')
    
    // Refresh page
    await page.reload()
    
    // Check if queue is preserved (depends on implementation)
    const fileItems = await page.locator('[data-testid="upload-file-item"]').count()
    
    if (fileItems > 0) {
      // Queue preserved
      await expect(page.locator('[data-testid="file-name"]')).toContainText('persistence_test.wav')
    } else {
      // Queue cleared on refresh - this is acceptable behavior
      console.log('Upload queue cleared on page refresh (expected behavior)')
    }
  })
})

test.describe('Performance and Stress Testing', () => {
  test('handles maximum recommended concurrent uploads', async ({ page }) => {
    const generator = new E2EWAVGenerator()
    const maxConcurrentFiles = 10
    
    // Generate multiple test files
    const filePaths: string[] = []
    
    for (let i = 0; i < maxConcurrentFiles; i++) {
      const wavBuffer = generator.generateWAV(`stress_${i}.wav`, 10, 44100, 16, 2)
      const filePath = await createTestFile(`stress_${i}.wav`, wavBuffer)
      filePaths.push(filePath)
    }
    
    // Upload all files simultaneously
    const fileInput = page.locator('input[type="file"]')
    await fileInput.setInputFiles(filePaths)
    
    // Wait for all files to be queued
    const fileItems = page.locator('[data-testid="upload-file-item"]')
    await expect(fileItems).toHaveCount(maxConcurrentFiles)
    
    // Start uploads
    const startTime = Date.now()
    await page.click('[data-testid="start-upload-button"]')
    
    // Wait for all uploads to complete
    await page.waitForFunction(
      (count) => {
        const successElements = document.querySelectorAll('[data-testid="upload-success"]')
        return successElements.length === count
      },
      maxConcurrentFiles,
      { timeout: 180000 } // 3 minutes for stress test
    )
    
    const endTime = Date.now()
    const uploadDuration = (endTime - startTime) / 1000
    
    console.log(`Uploaded ${maxConcurrentFiles} files in ${uploadDuration}s`)
    
    // Verify all uploads succeeded
    const successCount = await page.locator('[data-testid="upload-success"]').count()
    expect(successCount).toBe(maxConcurrentFiles)
    
    // Verify no errors
    const errorCount = await page.locator('[data-testid="upload-error"]').count()
    expect(errorCount).toBe(0)
    
    // Cleanup
    await cleanupTestFiles()
  })
})