export type UploadStatus = 'queued' | 'checking' | 'uploading' | 'success' | 'error' | 'duplicate'

export interface UploadFile {
  file: File
  id: string
  status: UploadStatus
  progress: number
  error?: string
}

export interface UploadQueueState {
  files: UploadFile[]
  isProcessing: boolean
}

export interface PresignedURLResponse {
  uploadUrl: string
  isDuplicate: boolean
  error?: string
  message?: string
}

export interface BatchPresignedURLRequest {
  filename: string
  fileSize: number
}

export interface BatchPresignedURLResponse {
  results: Record<string, PresignedURLResponse>
}

export interface UploadCallbacks {
  onProgress?: (progress: number) => void
  onSuccess?: () => void
  onError?: (error: Error) => void
}

export interface FileValidationResult {
  isValid: boolean
  error?: string
}

export interface UploadMetadata {
  uploadedAt: Date
  processingTime?: number
  fileSize: number
  originalName: string
}