import { UPLOAD_CONFIG } from './constants'
import { FileValidationResult } from '@/types'

export function generateFileId(): string {
  return `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`
}

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  
  const units = ['B', 'KB', 'MB', 'GB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  
  return `${Math.round(bytes / Math.pow(k, i))} ${units[i]}`
}

export function validateFile(file: File): FileValidationResult {
  // Check file extension
  const extension = `.${file.name.split('.').pop()?.toLowerCase()}`
  if (!UPLOAD_CONFIG.ALLOWED_EXTENSIONS.includes(extension)) {
    return {
      isValid: false,
      error: `Invalid file type. Only ${UPLOAD_CONFIG.ALLOWED_EXTENSIONS.join(', ')} files are allowed.`
    }
  }
  
  // Check file size
  if (file.size > UPLOAD_CONFIG.MAX_FILE_SIZE) {
    return {
      isValid: false,
      error: `File is too large. Maximum size is ${formatFileSize(UPLOAD_CONFIG.MAX_FILE_SIZE)}.`
    }
  }
  
  return { isValid: true }
}

export function filterValidFiles(files: File[]): File[] {
  return files.filter(file => {
    const validation = validateFile(file)
    if (!validation.isValid) {
      console.warn(`File ${file.name} rejected: ${validation.error}`)
    }
    return validation.isValid
  })
}

export function getFileExtension(filename: string): string {
  return filename.slice(filename.lastIndexOf('.'))
}

export function getFileNameWithoutExtension(filename: string): string {
  return filename.slice(0, filename.lastIndexOf('.'))
}