/**
 * File chunking utilities for large file uploads
 * Uses 4MB chunks for optimal Raspberry Pi performance
 */

export const CHUNK_SIZE = 4 * 1024 * 1024 // 4MB chunks

export interface FileChunk {
  index: number
  start: number
  end: number
  blob: Blob
  size: number
}

export interface ChunkedFile {
  file: File
  totalChunks: number
  chunks: FileChunk[]
}

/**
 * Splits a file into chunks for upload
 * Uses Blob.slice() to avoid loading entire file into memory
 */
export function chunkFile(file: File): ChunkedFile {
  const chunks: FileChunk[] = []
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  
  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const blob = file.slice(start, end)
    
    chunks.push({
      index: i,
      start,
      end,
      blob,
      size: end - start
    })
  }
  
  return {
    file,
    totalChunks,
    chunks
  }
}

/**
 * Calculates overall progress from chunk progress
 */
export function calculateChunkProgress(
  completedChunks: number,
  totalChunks: number,
  currentChunkProgress: number = 0
): number {
  if (totalChunks === 0) return 0
  
  const baseProgress = (completedChunks / totalChunks) * 100
  const currentProgress = (currentChunkProgress / totalChunks)
  
  return Math.round(baseProgress + currentProgress)
}

/**
 * Gets chunk info for resume capability
 */
export interface ChunkState {
  fileId: string
  fileName: string
  totalChunks: number
  completedChunks: number[]
  uploadId?: string // MinIO multipart upload ID
  lastActivity: number
}

/**
 * Stores chunk state in localStorage for resume
 */
export function saveChunkState(state: ChunkState): void {
  const key = `chunk_${state.fileId}`
  localStorage.setItem(key, JSON.stringify(state))
}

/**
 * Retrieves chunk state for resume
 */
export function getChunkState(fileId: string): ChunkState | null {
  const key = `chunk_${fileId}`
  const data = localStorage.getItem(key)
  
  if (!data) return null
  
  try {
    const state = JSON.parse(data) as ChunkState
    
    // Check if state is too old (24 hours)
    const dayInMs = 24 * 60 * 60 * 1000
    if (Date.now() - state.lastActivity > dayInMs) {
      localStorage.removeItem(key)
      return null
    }
    
    return state
  } catch {
    localStorage.removeItem(key)
    return null
  }
}

/**
 * Clears chunk state after successful upload
 */
export function clearChunkState(fileId: string): void {
  const key = `chunk_${fileId}`
  localStorage.removeItem(key)
}

/**
 * Generates a unique file ID based on name and size
 * Used for resume capability
 */
export function generateFileHash(file: File): string {
  return `${file.name}_${file.size}_${file.lastModified}`
}

/**
 * Checks if we can resume an upload
 */
export function canResumeUpload(file: File): ChunkState | null {
  const fileId = generateFileHash(file)
  const state = getChunkState(fileId)
  
  if (!state) return null
  
  // Verify file matches
  if (state.fileName !== file.name) return null
  
  return state
}