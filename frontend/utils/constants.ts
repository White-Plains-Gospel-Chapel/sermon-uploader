export const UPLOAD_CONFIG = {
  MAX_FILE_SIZE: 2 * 1024 * 1024 * 1024, // 2GB
  ALLOWED_EXTENSIONS: ['.wav'] as string[],
  ALLOWED_MIME_TYPES: ['audio/wav', 'audio/wave'] as string[],
  BATCH_SIZE: 20,
  RETRY_ATTEMPTS: 3,
  RETRY_DELAY: 1000,
}

export const API_ENDPOINTS = {
  BASE_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000',
  HEALTH: '/api/health',
  PRESIGNED_URL: '/api/upload/presigned',
  PRESIGNED_BATCH: '/api/upload/presigned-batch',
  COMPLETE_UPLOAD: '/api/upload/complete',
  CHECK_DUPLICATE: '/api/check-duplicate',
} as const

export const UI_TEXT = {
  UPLOAD: {
    TITLE: 'Sermon Upload',
    SUBTITLE: 'Upload WAV audio files for processing',
    DRAG_ACTIVE: 'Release to upload',
    DRAG_INACTIVE: 'Drag & drop your WAV files',
    BROWSE: 'click here to browse',
  },
  STATUS: {
    QUEUED: 'Queued',
    CHECKING: 'Checking',
    UPLOADING: 'Uploading',
    SUCCESS: 'Complete',
    DUPLICATE: 'Duplicate',
    ERROR: 'Failed',
  },
  ERROR: {
    GENERIC: 'Upload failed',
    DUPLICATE: 'File already exists',
    INVALID_TYPE: 'Invalid file type. Only WAV files are allowed.',
    FILE_TOO_LARGE: 'File is too large. Maximum size is 2GB.',
  },
} as const