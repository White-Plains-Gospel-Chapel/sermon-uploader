export interface ApiResponse<T = any> {
  success: boolean
  data?: T
  error?: string
  message?: string
}

export interface HealthCheckResponse {
  status: string
  minio: boolean
  fileCount: number
}

export interface DuplicateCheckResponse {
  isDuplicate: boolean
  existingFile?: string
}

export interface CompleteUploadRequest {
  filename: string
}

export interface CompleteUploadResponse {
  success: boolean
  message: string
  metadata?: {
    duration: number
    processingTime: number
  }
}

export interface ApiError extends Error {
  code?: string
  statusCode?: number
  details?: any
}