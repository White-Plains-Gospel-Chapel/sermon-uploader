/**
 * Audio Validation Tests
 * Tests for validating WAV file integrity and format compliance
 */

import { validateFile, validateWAVHeader, extractWAVMetadata } from '@/lib/validation'
import { TestWAVGenerator } from '../upload/AudioUploadIntegrity.test'
import crypto from 'crypto'

describe('Audio Validation Tests', () => {
  const generator = new TestWAVGenerator()

  describe('WAV Header Validation', () => {
    test('validates correct WAV header structure', async () => {
      const wavBuffer = generator.generateWAV('valid_header.wav', 10, 44100, 16, 2)
      const arrayBuffer = wavBuffer.slice(0) // Create copy to test immutability
      
      const validation = validateWAVHeader(arrayBuffer)
      
      expect(validation.isValid).toBe(true)
      expect(validation.format).toBe('PCM')
      expect(validation.sampleRate).toBe(44100)
      expect(validation.bitDepth).toBe(16)
      expect(validation.channels).toBe(2)
      expect(validation.error).toBeUndefined()
      
      // Verify original buffer was not modified
      expect(arrayBuffer.byteLength).toBe(wavBuffer.byteLength)
    })

    test('validates different WAV formats correctly', async () => {
      const testCases = [
        { sampleRate: 22050, bitDepth: 16, channels: 1, name: 'mono_low' },
        { sampleRate: 44100, bitDepth: 16, channels: 2, name: 'cd_quality' },
        { sampleRate: 48000, bitDepth: 24, channels: 2, name: 'studio_quality' },
        { sampleRate: 96000, bitDepth: 24, channels: 2, name: 'high_res' },
        { sampleRate: 192000, bitDepth: 24, channels: 2, name: 'ultra_high_res' }
      ]
      
      testCases.forEach(testCase => {
        const wavBuffer = generator.generateWAV(
          `${testCase.name}.wav`, 5, testCase.sampleRate, testCase.bitDepth, testCase.channels
        )
        
        const validation = validateWAVHeader(wavBuffer)
        
        expect(validation.isValid).toBe(true)
        expect(validation.sampleRate).toBe(testCase.sampleRate)
        expect(validation.bitDepth).toBe(testCase.bitDepth)
        expect(validation.channels).toBe(testCase.channels)
        expect(validation.format).toBe('PCM')
      })
    })

    test('rejects corrupted WAV headers', async () => {
      const validBuffer = generator.generateWAV('valid.wav', 5, 44100, 16, 2)
      
      // Test various corruption scenarios
      const corruptionTests = [
        {
          name: 'missing_riff_signature',
          corrupt: (buffer: ArrayBuffer) => {
            const view = new DataView(buffer)
            // Corrupt RIFF signature
            view.setUint8(0, 0xFF)
            view.setUint8(1, 0xFF)
            view.setUint8(2, 0xFF)
            view.setUint8(3, 0xFF)
            return buffer
          }
        },
        {
          name: 'missing_wave_signature', 
          corrupt: (buffer: ArrayBuffer) => {
            const view = new DataView(buffer)
            // Corrupt WAVE signature
            view.setUint8(8, 0x00)
            view.setUint8(9, 0x00)
            view.setUint8(10, 0x00)
            view.setUint8(11, 0x00)
            return buffer
          }
        },
        {
          name: 'invalid_format',
          corrupt: (buffer: ArrayBuffer) => {
            const view = new DataView(buffer)
            // Set invalid audio format (non-PCM)
            view.setUint16(20, 0x1234, true)
            return buffer
          }
        },
        {
          name: 'missing_fmt_chunk',
          corrupt: (buffer: ArrayBuffer) => {
            const view = new DataView(buffer)
            // Corrupt fmt chunk signature
            view.setUint8(12, 0x00)
            view.setUint8(13, 0x00)
            view.setUint8(14, 0x00)
            view.setUint8(15, 0x00)
            return buffer
          }
        },
        {
          name: 'missing_data_chunk',
          corrupt: (buffer: ArrayBuffer) => {
            const view = new DataView(buffer)
            // Corrupt data chunk signature
            view.setUint8(36, 0x00)
            view.setUint8(37, 0x00)
            view.setUint8(38, 0x00)
            view.setUint8(39, 0x00)
            return buffer
          }
        }
      ]
      
      corruptionTests.forEach(test => {
        const corruptedBuffer = test.corrupt(validBuffer.slice(0))
        const validation = validateWAVHeader(corruptedBuffer)
        
        expect(validation.isValid).toBe(false)
        expect(validation.error).toBeTruthy()
        expect(validation.error).toMatch(/invalid|missing|corrupted/i)
      })
    })

    test('validates file size consistency', async () => {
      const testDurations = [1, 10, 30, 60, 300] // 1 sec to 5 minutes
      
      testDurations.forEach(duration => {
        const wavBuffer = generator.generateWAV(`duration_${duration}.wav`, duration, 44100, 16, 2)
        const validation = validateWAVHeader(wavBuffer)
        
        expect(validation.isValid).toBe(true)
        
        // Calculate expected file size
        const expectedSamples = duration * 44100 * 2 // stereo
        const expectedDataSize = expectedSamples * 2 // 16-bit
        const expectedFileSize = 44 + expectedDataSize // header + data
        
        expect(wavBuffer.byteLength).toBe(expectedFileSize)
        expect(validation.fileSize).toBe(expectedFileSize)
        expect(validation.duration).toBeCloseTo(duration, 1)
      })
    })
  })

  describe('WAV Metadata Extraction', () => {
    test('extracts complete metadata without altering file', async () => {
      const sampleRate = 48000
      const bitDepth = 24
      const channels = 2
      const duration = 45
      
      const originalBuffer = generator.generateWAV('metadata_test.wav', duration, sampleRate, bitDepth, channels)
      const originalHash = crypto.createHash('sha256').update(new Uint8Array(originalBuffer)).digest('hex')
      
      const metadata = extractWAVMetadata(originalBuffer)
      
      // Verify metadata accuracy
      expect(metadata.format).toBe('PCM')
      expect(metadata.sampleRate).toBe(sampleRate)
      expect(metadata.bitDepth).toBe(bitDepth)
      expect(metadata.channels).toBe(channels)
      expect(metadata.duration).toBeCloseTo(duration, 1)
      expect(metadata.fileSize).toBe(originalBuffer.byteLength)
      expect(metadata.isLossless).toBe(true)
      
      // Calculate expected bitrate
      const expectedBitrate = sampleRate * bitDepth * channels
      expect(metadata.bitrate).toBe(expectedBitrate)
      
      // Verify original buffer unchanged
      const postExtractionHash = crypto.createHash('sha256').update(new Uint8Array(originalBuffer)).digest('hex')
      expect(postExtractionHash).toBe(originalHash)
    })

    test('calculates audio quality metrics accurately', async () => {
      const qualityTests = [
        { 
          sampleRate: 44100, bitDepth: 16, channels: 2, 
          expectedQuality: 'CD Quality', expectedBitrate: 1411200 
        },
        { 
          sampleRate: 48000, bitDepth: 16, channels: 2, 
          expectedQuality: 'Broadcast Quality', expectedBitrate: 1536000 
        },
        { 
          sampleRate: 96000, bitDepth: 24, channels: 2, 
          expectedQuality: 'High Resolution', expectedBitrate: 4608000 
        },
        { 
          sampleRate: 192000, bitDepth: 24, channels: 2, 
          expectedQuality: 'Ultra High Resolution', expectedBitrate: 9216000 
        }
      ]
      
      qualityTests.forEach(test => {
        const buffer = generator.generateWAV('quality_test.wav', 10, test.sampleRate, test.bitDepth, test.channels)
        const metadata = extractWAVMetadata(buffer)
        
        expect(metadata.quality).toBe(test.expectedQuality)
        expect(metadata.bitrate).toBe(test.expectedBitrate)
        expect(metadata.isLossless).toBe(true)
        expect(metadata.compressionRatio).toBe(1.0) // No compression
      })
    })
  })

  describe('File Size and Integrity Validation', () => {
    test('validates file size limits for practical upload', async () => {
      // Test various file sizes
      const sizeTests = [
        { duration: 1, description: 'very_small' },    // ~170KB
        { duration: 60, description: 'small' },        // ~10MB  
        { duration: 600, description: 'large' },       // ~100MB
        { duration: 3600, description: 'very_large' }  // ~600MB
      ]
      
      sizeTests.forEach(test => {
        const buffer = generator.generateWAV(`${test.description}.wav`, test.duration, 44100, 16, 2)
        const validation = validateWAVHeader(buffer)
        
        expect(validation.isValid).toBe(true)
        expect(validation.fileSize).toBeGreaterThan(0)
        expect(validation.duration).toBeCloseTo(test.duration, 1)
        
        // Verify size calculation accuracy
        const expectedSamples = test.duration * 44100 * 2
        const expectedDataSize = expectedSamples * 2
        const expectedTotalSize = 44 + expectedDataSize
        
        expect(buffer.byteLength).toBe(expectedTotalSize)
      })
    })

    test('detects truncated or incomplete files', async () => {
      const completeBuffer = generator.generateWAV('complete.wav', 30, 44100, 16, 2)
      
      // Create truncated versions
      const truncationTests = [
        { size: 20, description: 'header_only' },           // Only partial header
        { size: 44, description: 'header_no_data' },        // Header but no data
        { size: Math.floor(completeBuffer.byteLength / 2), description: 'half_file' }  // Half the file
      ]
      
      truncationTests.forEach(test => {
        const truncatedBuffer = completeBuffer.slice(0, test.size)
        const validation = validateWAVHeader(truncatedBuffer)
        
        if (test.size < 44) {
          // Incomplete header
          expect(validation.isValid).toBe(false)
          expect(validation.error).toMatch(/incomplete|truncated|header/i)
        } else {
          // May have valid header but incomplete data
          expect(validation.isValid).toBe(false)
          expect(validation.error).toMatch(/incomplete|truncated|data/i)
        }
      })
    })

    test('validates data chunk size consistency', async () => {
      const buffer = generator.generateWAV('consistency_test.wav', 60, 44100, 16, 2)
      const view = new DataView(buffer)
      
      // Extract sizes from header
      const fileSize = view.getUint32(4, true) + 8 // RIFF chunk size + 8
      const dataChunkSize = view.getUint32(40, true)
      
      // Verify consistency
      const expectedDataSize = fileSize - 44 // Total - header
      expect(dataChunkSize).toBe(expectedDataSize)
      expect(buffer.byteLength).toBe(fileSize)
      
      // Test with manipulated header
      const manipulatedBuffer = buffer.slice(0)
      const manipulatedView = new DataView(manipulatedBuffer)
      
      // Set incorrect data chunk size
      manipulatedView.setUint32(40, dataChunkSize * 2, true)
      
      const validation = validateWAVHeader(manipulatedBuffer)
      expect(validation.isValid).toBe(false)
      expect(validation.error).toMatch(/size|inconsistent|mismatch/i)
    })
  })

  describe('Bit Depth and Sample Rate Validation', () => {
    test('supports all standard audio bit depths', async () => {
      const supportedBitDepths = [16, 24] // Standard bit depths for WAV
      
      supportedBitDepths.forEach(bitDepth => {
        const buffer = generator.generateWAV(`bitdepth_${bitDepth}.wav`, 10, 44100, bitDepth, 2)
        const validation = validateWAVHeader(buffer)
        
        expect(validation.isValid).toBe(true)
        expect(validation.bitDepth).toBe(bitDepth)
        expect(validation.isLossless).toBe(true)
        
        // Verify dynamic range
        const dynamicRange = Math.pow(2, bitDepth)
        expect(validation.dynamicRange).toBe(dynamicRange)
      })
    })

    test('supports all standard sample rates', async () => {
      const supportedSampleRates = [22050, 44100, 48000, 88200, 96000, 176400, 192000]
      
      supportedSampleRates.forEach(sampleRate => {
        const buffer = generator.generateWAV(`samplerate_${sampleRate}.wav`, 5, sampleRate, 16, 2)
        const validation = validateWAVHeader(buffer)
        
        expect(validation.isValid).toBe(true)
        expect(validation.sampleRate).toBe(sampleRate)
        expect(validation.nyquistFrequency).toBe(sampleRate / 2)
      })
    })

    test('validates channel configurations', async () => {
      const channelConfigs = [
        { channels: 1, description: 'mono' },
        { channels: 2, description: 'stereo' }
      ]
      
      channelConfigs.forEach(config => {
        const buffer = generator.generateWAV(`${config.description}.wav`, 10, 44100, 16, config.channels)
        const validation = validateWAVHeader(buffer)
        
        expect(validation.isValid).toBe(true)
        expect(validation.channels).toBe(config.channels)
        expect(validation.channelConfig).toBe(config.description)
      })
    })
  })

  describe('Audio Content Validation', () => {
    test('validates audio data integrity', async () => {
      const buffer = generator.generateWAV('integrity_test.wav', 30, 44100, 16, 2)
      
      // Extract audio data section
      const dataStart = 44
      const audioData = buffer.slice(dataStart)
      
      expect(audioData.byteLength).toBeGreaterThan(0)
      
      // Verify audio data is not all zeros (silent)
      const audioArray = new Int16Array(audioData)
      const nonZeroSamples = Array.from(audioArray).filter(sample => sample !== 0)
      
      expect(nonZeroSamples.length).toBeGreaterThan(0)
      expect(nonZeroSamples.length / audioArray.length).toBeGreaterThan(0.5) // At least 50% non-zero
    })

    test('validates audio level and clipping detection', async () => {
      const buffer = generator.generateWAV('levels_test.wav', 10, 44100, 16, 2)
      
      const metadata = extractWAVMetadata(buffer)
      
      // Check for clipping (samples at maximum values)
      expect(metadata.hasClipping).toBe(false)
      
      // Verify peak levels are reasonable
      expect(metadata.peakLevel).toBeLessThan(32767) // 16-bit max
      expect(metadata.peakLevel).toBeGreaterThan(0)
      
      // RMS should be significantly lower than peak
      expect(metadata.rmsLevel).toBeLessThan(metadata.peakLevel)
      expect(metadata.rmsLevel).toBeGreaterThan(0)
    })

    test('detects silence and validates dynamic range', async () => {
      // Create a file with known dynamic content
      const buffer = generator.generateWAV('dynamic_test.wav', 20, 44100, 16, 2)
      const metadata = extractWAVMetadata(buffer)
      
      expect(metadata.isSilent).toBe(false)
      expect(metadata.dynamicRange).toBeGreaterThan(20) // dB
      expect(metadata.signalToNoise).toBeGreaterThan(40) // dB
    })
  })

  describe('Performance and Memory Efficiency', () => {
    test('validates large files efficiently without memory issues', async () => {
      if (process.env.NODE_ENV === 'test') {
        // Skip in normal test runs to avoid memory issues
        return
      }
      
      const largeBuffer = generator.generateWAV('large_performance.wav', 600, 96000, 24, 2) // ~500MB
      
      const startTime = Date.now()
      const startMemory = process.memoryUsage().heapUsed
      
      const validation = validateWAVHeader(largeBuffer)
      
      const endTime = Date.now()
      const endMemory = process.memoryUsage().heapUsed
      
      expect(validation.isValid).toBe(true)
      
      // Performance assertions
      expect(endTime - startTime).toBeLessThan(5000) // Under 5 seconds
      expect(endMemory - startMemory).toBeLessThan(largeBuffer.byteLength * 0.1) // Under 10% memory overhead
    })

    test('validates multiple files concurrently', async () => {
      const fileCount = 10
      const files = Array.from({ length: fileCount }, (_, i) => 
        generator.generateWAV(`concurrent_${i}.wav`, 10, 44100, 16, 2)
      )
      
      const startTime = Date.now()
      
      const validations = await Promise.all(
        files.map(buffer => Promise.resolve(validateWAVHeader(buffer)))
      )
      
      const endTime = Date.now()
      
      // All validations should succeed
      validations.forEach(validation => {
        expect(validation.isValid).toBe(true)
      })
      
      // Concurrent validation should be efficient
      expect(endTime - startTime).toBeLessThan(2000) // Under 2 seconds for 10 files
    })
  })
})