/**
 * File validation utilities for audio upload
 */

export interface WAVMetadata {
  sampleRate: number
  channels: number
  bitDepth: number
  duration: number
  fileSize: number
}

export interface ValidationResult {
  isValid: boolean
  error?: string
  metadata?: WAVMetadata
}

/**
 * Validate uploaded file
 */
export function validateFile(file: File): ValidationResult {
  // Check file type
  if (!file.type.includes('audio/wav') && !file.name.toLowerCase().endsWith('.wav')) {
    return {
      isValid: false,
      error: 'Only WAV files are allowed'
    }
  }

  // Check file size (2GB limit)
  if (file.size > 2 * 1024 * 1024 * 1024) {
    return {
      isValid: false,
      error: 'File size exceeds 2GB limit'
    }
  }

  if (file.size === 0) {
    return {
      isValid: false,
      error: 'File is empty'
    }
  }

  return {
    isValid: true
  }
}

/**
 * Validate WAV file header
 */
export async function validateWAVHeader(file: File): Promise<ValidationResult> {
  try {
    const headerBuffer = await readFileHeader(file, 44) // WAV header is 44 bytes
    const metadata = extractWAVMetadata(headerBuffer)
    
    if (!metadata) {
      return {
        isValid: false,
        error: 'Invalid WAV file header'
      }
    }

    return {
      isValid: true,
      metadata
    }
  } catch (error) {
    return {
      isValid: false,
      error: error instanceof Error ? error.message : 'Failed to validate WAV header'
    }
  }
}

/**
 * Extract WAV metadata from header buffer
 */
export function extractWAVMetadata(buffer: ArrayBuffer): WAVMetadata | null {
  try {
    const view = new DataView(buffer)
    
    // Check RIFF header
    const riffHeader = String.fromCharCode(...new Uint8Array(buffer.slice(0, 4)))
    if (riffHeader !== 'RIFF') {
      return null
    }
    
    // Check WAVE format
    const waveHeader = String.fromCharCode(...new Uint8Array(buffer.slice(8, 12)))
    if (waveHeader !== 'WAVE') {
      return null
    }
    
    // Extract format chunk data (starting at byte 16)
    const channels = view.getUint16(22, true) // little-endian
    const sampleRate = view.getUint32(24, true)
    const bitDepth = view.getUint16(34, true)
    
    // Calculate duration (approximate, would need actual data chunk size)
    const fileSize = view.getUint32(4, true) + 8
    const bytesPerSample = (bitDepth / 8) * channels
    const dataSize = fileSize - 44 // Approximate
    const duration = dataSize / (sampleRate * bytesPerSample)
    
    return {
      channels,
      sampleRate,
      bitDepth,
      duration,
      fileSize
    }
  } catch (error) {
    return null
  }
}

/**
 * Read file header bytes
 */
function readFileHeader(file: File, bytes: number): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(new Error('Failed to read file header'))
    reader.readAsArrayBuffer(file.slice(0, bytes))
  })
}