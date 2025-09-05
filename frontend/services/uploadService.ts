import { api } from '@/lib/api'

// Re-export the existing api functions with a cleaner interface
export const uploadService = {
  getPresignedURL: api.getPresignedURL,
  getPresignedURLsBatch: api.getPresignedURLsBatch,
  uploadToMinIO: api.uploadToMinIO,
  completeUpload: api.completeUpload,
  completeUploadBatch: api.completeUploadBatch,
  checkDuplicate: api.checkDuplicate
}