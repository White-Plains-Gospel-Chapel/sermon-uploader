/**
 * Chunked Uploader for Large Files
 * Implements multipart upload with 5MB chunks (MinIO minimum)
 * Server-controlled queue, conservative concurrency
 */

import { api } from './api';

// Constants based on our decisions
const CHUNK_SIZE = 5 * 1024 * 1024; // 5MB chunks (MinIO minimum)
const MAX_CONCURRENT_CHUNKS = 2; // 2 chunks in parallel per file
const MAX_RETRIES = 3;
const RETRY_DELAY_BASE = 1000; // Base delay for exponential backoff

// Types
export interface UploadProgress {
  uploadId: string;
  filename: string;
  totalBytes: number;
  uploadedBytes: number;
  percentage: number;
  chunksUploaded: number;
  totalChunks: number;
  speed: number; // bytes per second
  eta: number; // seconds remaining
  status: 'pending' | 'uploading' | 'completed' | 'error' | 'paused';
  error?: string;
}

export interface UploadOptions {
  onProgress?: (progress: UploadProgress) => void;
  onComplete?: (filename: string) => void;
  onError?: (error: Error) => void;
}

interface UploadSession {
  uploadId: string;
  filename: string;
  fileHash: string;
  totalParts: number;
  chunkSize: number;
  uploadedParts: CompletedPart[];
  startTime: number;
  bytesUploaded: number;
}

interface CompletedPart {
  partNumber: number;
  etag: string;
  size: number;
}

/**
 * ChunkedUploader - Handles large file uploads with chunking
 */
export class ChunkedUploader {
  private currentUpload: UploadSession | null = null;
  private uploadSpeed = 0;
  private lastProgressTime = 0;
  private lastProgressBytes = 0;
  private abortController: AbortController | null = null;

  constructor(private options: UploadOptions = {}) {}

  /**
   * Upload a file using multipart upload
   */
  async uploadFile(file: File): Promise<void> {
    try {
      // Calculate file hash for deduplication
      const fileHash = await this.calculateFileHash(file);

      // Check for existing session (resumability)
      const existingSession = await this.checkForExistingSession(fileHash);
      
      if (existingSession) {
        // Resume existing upload
        await this.resumeUpload(file, existingSession);
      } else {
        // Start new upload
        await this.startNewUpload(file, fileHash);
      }
    } catch (error) {
      this.handleError(error as Error);
      throw error;
    }
  }

  /**
   * Start a new multipart upload
   */
  private async startNewUpload(file: File, fileHash: string): Promise<void> {
    // Initialize multipart upload
    const initResponse = await this.retryWithBackoff(() =>
      api.post('/api/upload/multipart/init', {
        filename: file.name,
        fileSize: file.size,
        chunkSize: CHUNK_SIZE,
        fileHash: fileHash,
      })
    );

    const { uploadId, totalParts, chunkSize } = initResponse.data;

    // Create upload session
    this.currentUpload = {
      uploadId,
      filename: file.name,
      fileHash,
      totalParts,
      chunkSize,
      uploadedParts: [],
      startTime: Date.now(),
      bytesUploaded: 0,
    };

    // Save session to localStorage for resumability
    this.saveSession(this.currentUpload);

    // Upload all chunks
    await this.uploadChunks(file);

    // Complete the upload
    await this.completeUpload();
  }

  /**
   * Resume an existing upload
   */
  private async resumeUpload(file: File, session: UploadSession): Promise<void> {
    this.currentUpload = session;

    // Get list of already uploaded parts
    const partsResponse = await api.get('/api/upload/multipart/parts', {
      params: { uploadId: session.uploadId },
    });

    const uploadedParts = partsResponse.data.uploadedParts || [];
    this.currentUpload.uploadedParts = uploadedParts;

    // Calculate bytes already uploaded
    this.currentUpload.bytesUploaded = uploadedParts.reduce(
      (sum: number, part: CompletedPart) => sum + part.size,
      0
    );

    // Continue uploading remaining chunks
    await this.uploadChunks(file);

    // Complete the upload
    await this.completeUpload();
  }

  /**
   * Upload file chunks with concurrency control
   */
  private async uploadChunks(file: File): Promise<void> {
    if (!this.currentUpload) throw new Error('No upload session');

    const { uploadId, totalParts, chunkSize, uploadedParts } = this.currentUpload;
    
    // Determine which parts still need uploading
    const uploadedPartNumbers = new Set(uploadedParts.map(p => p.partNumber));
    const partsToUpload: number[] = [];
    
    for (let i = 1; i <= totalParts; i++) {
      if (!uploadedPartNumbers.has(i)) {
        partsToUpload.push(i);
      }
    }

    if (partsToUpload.length === 0) {
      // All parts already uploaded
      return;
    }

    // Create abort controller for cancellation
    this.abortController = new AbortController();

    // Upload chunks with concurrency limit
    const uploadQueue = [...partsToUpload];
    const activeUploads: Promise<void>[] = [];

    while (uploadQueue.length > 0 || activeUploads.length > 0) {
      // Start new uploads up to concurrency limit
      while (activeUploads.length < MAX_CONCURRENT_CHUNKS && uploadQueue.length > 0) {
        const partNumber = uploadQueue.shift()!;
        const uploadPromise = this.uploadChunk(file, partNumber)
          .then(() => {
            // Remove from active uploads
            const index = activeUploads.indexOf(uploadPromise);
            if (index > -1) activeUploads.splice(index, 1);
          })
          .catch((error) => {
            // On error, re-queue the part
            if (error.name !== 'AbortError') {
              uploadQueue.push(partNumber);
            }
            const index = activeUploads.indexOf(uploadPromise);
            if (index > -1) activeUploads.splice(index, 1);
          });
        
        activeUploads.push(uploadPromise);
      }

      // Wait for at least one upload to complete
      if (activeUploads.length > 0) {
        await Promise.race(activeUploads);
      }
    }
  }

  /**
   * Upload a single chunk with retry logic
   */
  private async uploadChunk(file: File, partNumber: number): Promise<void> {
    if (!this.currentUpload) throw new Error('No upload session');

    const { uploadId, chunkSize } = this.currentUpload;
    
    // Calculate chunk boundaries
    const start = (partNumber - 1) * chunkSize;
    const end = Math.min(start + chunkSize, file.size);
    const chunk = file.slice(start, end);

    // Get presigned URL for this part
    const presignedResponse = await this.retryWithBackoff(() =>
      api.get('/api/upload/multipart/presigned', {
        params: { uploadId, partNumber },
        signal: this.abortController?.signal,
      })
    );

    const { url } = presignedResponse.data;

    // Upload the chunk
    const uploadStartTime = Date.now();
    const uploadResponse = await this.retryWithBackoff(() =>
      fetch(url, {
        method: 'PUT',
        body: chunk,
        headers: {
          'Content-Type': 'application/octet-stream',
        },
        signal: this.abortController?.signal,
      })
    );

    if (!uploadResponse.ok) {
      throw new Error(`Failed to upload part ${partNumber}: ${uploadResponse.statusText}`);
    }

    const uploadEndTime = Date.now();
    const chunkUploadTime = (uploadEndTime - uploadStartTime) / 1000; // seconds
    
    // Parse response
    const result = await uploadResponse.json();
    
    // Record uploaded part
    this.currentUpload.uploadedParts.push({
      partNumber,
      etag: result.etag,
      size: chunk.size,
    });
    
    this.currentUpload.bytesUploaded += chunk.size;

    // Update progress
    this.updateProgress(chunk.size, chunkUploadTime);
    
    // Save session after each successful chunk
    this.saveSession(this.currentUpload);
  }

  /**
   * Complete the multipart upload
   */
  private async completeUpload(): Promise<void> {
    if (!this.currentUpload) throw new Error('No upload session');

    const { uploadId, uploadedParts, filename } = this.currentUpload;

    // Sort parts by part number
    const sortedParts = [...uploadedParts].sort((a, b) => a.partNumber - b.partNumber);

    // Complete the upload
    await this.retryWithBackoff(() =>
      api.post('/api/upload/multipart/complete', {
        uploadId,
        parts: sortedParts,
      })
    );

    // Clear session
    this.clearSession(this.currentUpload.fileHash);
    
    // Notify completion
    if (this.options.onComplete) {
      this.options.onComplete(filename);
    }

    // Final progress update
    this.updateProgress(0, 0, 'completed');
  }

  /**
   * Update and report progress
   */
  private updateProgress(
    bytesJustUploaded: number,
    timeForChunk: number,
    status?: UploadProgress['status']
  ): void {
    if (!this.currentUpload || !this.options.onProgress) return;

    const now = Date.now();
    
    // Calculate upload speed
    if (timeForChunk > 0) {
      const chunkSpeed = bytesJustUploaded / timeForChunk; // bytes per second
      // Smooth speed calculation with moving average
      this.uploadSpeed = this.uploadSpeed === 0 
        ? chunkSpeed 
        : (this.uploadSpeed * 0.7 + chunkSpeed * 0.3);
    }

    // Calculate ETA
    const bytesRemaining = this.currentUpload.totalParts * this.currentUpload.chunkSize - 
                          this.currentUpload.bytesUploaded;
    const eta = this.uploadSpeed > 0 ? bytesRemaining / this.uploadSpeed : 0;

    // Calculate percentage
    const totalBytes = this.currentUpload.totalParts * this.currentUpload.chunkSize;
    const percentage = (this.currentUpload.bytesUploaded / totalBytes) * 100;

    const progress: UploadProgress = {
      uploadId: this.currentUpload.uploadId,
      filename: this.currentUpload.filename,
      totalBytes,
      uploadedBytes: this.currentUpload.bytesUploaded,
      percentage: Math.min(percentage, 100),
      chunksUploaded: this.currentUpload.uploadedParts.length,
      totalChunks: this.currentUpload.totalParts,
      speed: this.uploadSpeed,
      eta: Math.round(eta),
      status: status || 'uploading',
    };

    this.options.onProgress(progress);
  }

  /**
   * Calculate SHA-256 hash of file for deduplication
   */
  private async calculateFileHash(file: File): Promise<string> {
    // For large files, we'll hash the first 1MB, middle 1MB, and last 1MB
    // This is faster than hashing the entire file
    const chunkSize = 1024 * 1024; // 1MB
    const chunks: ArrayBuffer[] = [];

    // First chunk
    chunks.push(await file.slice(0, chunkSize).arrayBuffer());

    // Middle chunk (if file is large enough)
    if (file.size > chunkSize * 3) {
      const middleStart = Math.floor(file.size / 2) - chunkSize / 2;
      chunks.push(await file.slice(middleStart, middleStart + chunkSize).arrayBuffer());
    }

    // Last chunk (if file is large enough)
    if (file.size > chunkSize * 2) {
      chunks.push(await file.slice(file.size - chunkSize).arrayBuffer());
    }

    // Combine chunks
    const combinedBuffer = await new Blob(chunks).arrayBuffer();

    // Calculate hash
    const hashBuffer = await crypto.subtle.digest('SHA-256', combinedBuffer);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');

    // Include file size in hash for additional uniqueness
    return `${hashHex}-${file.size}`;
  }

  /**
   * Check for existing upload session
   */
  private async checkForExistingSession(fileHash: string): Promise<UploadSession | null> {
    const sessionKey = `upload_session_${fileHash}`;
    const sessionData = localStorage.getItem(sessionKey);
    
    if (!sessionData) return null;

    try {
      const session = JSON.parse(sessionData) as UploadSession;
      
      // Check if session is still valid (less than 24 hours old)
      const age = Date.now() - session.startTime;
      if (age > 24 * 60 * 60 * 1000) {
        // Session too old, remove it
        localStorage.removeItem(sessionKey);
        return null;
      }

      // Verify session with backend
      const response = await api.get('/api/upload/multipart/parts', {
        params: { uploadId: session.uploadId },
      });

      if (response.data) {
        return session;
      }
    } catch (error) {
      // Session invalid, remove it
      localStorage.removeItem(sessionKey);
    }

    return null;
  }

  /**
   * Save upload session to localStorage
   */
  private saveSession(session: UploadSession): void {
    const sessionKey = `upload_session_${session.fileHash}`;
    localStorage.setItem(sessionKey, JSON.stringify(session));
  }

  /**
   * Clear upload session from localStorage
   */
  private clearSession(fileHash: string): void {
    const sessionKey = `upload_session_${fileHash}`;
    localStorage.removeItem(sessionKey);
  }

  /**
   * Retry with exponential backoff
   */
  private async retryWithBackoff<T>(
    fn: () => Promise<T>,
    attempt = 1
  ): Promise<T> {
    try {
      return await fn();
    } catch (error: any) {
      // Don't retry on certain errors
      if (
        error.response?.status === 401 || // Unauthorized
        error.response?.status === 403 || // Forbidden
        error.response?.status === 409 || // Conflict (duplicate)
        error.name === 'AbortError' // User cancelled
      ) {
        throw error;
      }

      if (attempt >= MAX_RETRIES) {
        throw error;
      }

      // Calculate delay with exponential backoff and jitter
      const delay = Math.min(
        RETRY_DELAY_BASE * Math.pow(2, attempt - 1) + Math.random() * 1000,
        30000 // Max 30 seconds
      );

      console.log(`Retry attempt ${attempt}/${MAX_RETRIES} after ${delay}ms`);
      await this.sleep(delay);

      return this.retryWithBackoff(fn, attempt + 1);
    }
  }

  /**
   * Sleep helper
   */
  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Handle errors
   */
  private handleError(error: Error): void {
    console.error('Upload error:', error);
    
    if (this.options.onError) {
      this.options.onError(error);
    }

    // Update progress with error status
    if (this.currentUpload && this.options.onProgress) {
      this.updateProgress(0, 0, 'error');
    }
  }

  /**
   * Cancel the current upload
   */
  cancelUpload(): void {
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    if (this.currentUpload) {
      // Keep session for resuming later
      this.updateProgress(0, 0, 'paused');
    }
  }

  /**
   * Get queue status (for server-controlled queue)
   */
  async getQueueStatus(): Promise<{ position: number; total: number }> {
    try {
      const response = await api.get('/api/upload/multipart/sessions');
      const sessions = response.data.sessions || [];
      
      // Find our position in queue
      let position = 0;
      if (this.currentUpload) {
        position = sessions.findIndex(
          (s: any) => s.uploadId === this.currentUpload?.uploadId
        ) + 1;
      }

      return {
        position: position || sessions.length + 1,
        total: sessions.length,
      };
    } catch (error) {
      return { position: 0, total: 0 };
    }
  }
}