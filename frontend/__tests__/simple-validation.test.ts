/**
 * Simple validation tests
 */

import { validateFile } from '@/lib/validation'

describe('Simple Validation Tests', () => {
  test('validates file types', () => {
    // Test valid WAV file with content
    const validFile = new File(['audio data'], 'test.wav', { type: 'audio/wav' })
    const result = validateFile(validFile)
    expect(result.isValid).toBe(true)
  })

  test('rejects non-WAV files', () => {
    const invalidFile = new File([''], 'test.mp3', { type: 'audio/mp3' })
    const result = validateFile(invalidFile)
    expect(result.isValid).toBe(false)
    expect(result.error).toBe('Only WAV files are allowed')
  })

  test('rejects empty files', () => {
    const emptyFile = new File([''], 'empty.wav', { type: 'audio/wav' })
    // Simulate empty file by setting size to 0
    Object.defineProperty(emptyFile, 'size', { value: 0 })
    const result = validateFile(emptyFile)
    expect(result.isValid).toBe(false)
    expect(result.error).toBe('File is empty')
  })
})