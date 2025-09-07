// API configuration
const getApiUrl = () => {
  // In production, use the same host as the frontend
  // In development, you might want to use a different port
  if (typeof window !== 'undefined') {
    // Client-side: use current host
    return `${window.location.protocol}//${window.location.host}`
  }
  // Server-side rendering (if needed)
  return process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000'
}

const API_BASE = getApiUrl()

export const api = {
  // Status endpoint
  async getStatus() {
    const response = await fetch(`${API_BASE}/api/status`)
    if (!response.ok) {
      throw new Error('Failed to fetch status')
    }
    return response.json()
  },

  // Upload files (legacy method)
  async uploadFiles(files: File[]) {
    const formData = new FormData()
    files.forEach(file => formData.append('files', file))

    const response = await fetch(`${API_BASE}/api/upload`, {
      method: 'POST',
      body: formData
    })

    if (!response.ok) {
      const error = await response.text()
      throw new Error(error || 'Upload failed')
    }

    return response.json()
  },

  // Get presigned URL for direct upload to MinIO (includes duplicate check)
  async getPresignedURL(filename: string, fileSize: number) {
    const response = await fetch(`${API_BASE}/api/upload/presigned`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ filename, fileSize })
    })

    const result = await response.json()

    if (!response.ok) {
      // Handle duplicate files specifically
      if (response.status === 409 && result.isDuplicate) {
        const error = new Error('File already exists') as Error & { isDuplicate: boolean; name: string }
        error.name = 'DuplicateFileError'
        error.isDuplicate = true
        throw error
      }
      throw new Error(result.message || 'Failed to get upload URL')
    }

    return result
  },

  // Get presigned URLs for multiple files at once (batch optimization)
  async getPresignedURLsBatch(files: Array<{filename: string, fileSize: number}>) {
    const response = await fetch(`${API_BASE}/api/upload/presigned-batch`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ 
        files: files.map(f => ({ filename: f.filename, fileSize: f.fileSize }))
      })
    })

    const result = await response.json()

    if (!response.ok) {
      throw new Error(result.message || 'Failed to get batch upload URLs')
    }

    return result
  },

  // Get unified dashboard data (status + recent files)
  async getDashboard(options?: { limit?: number, includeMetadata?: boolean }) {
    const params = new URLSearchParams()
    if (options?.limit) params.set('limit', options.limit.toString())
    if (options?.includeMetadata) params.set('metadata', 'true')
    
    const queryString = params.toString()
    const url = `${API_BASE}/api/dashboard${queryString ? '?' + queryString : ''}`
    
    const response = await fetch(url)
    if (!response.ok) {
      throw new Error('Failed to fetch dashboard data')
    }
    return response.json()
  },

  // Upload directly to MinIO using presigned URL (supports both CloudFlare and direct MinIO)
  async uploadToMinIO(file: File, presignedURL: string, onProgress?: (progress: number) => void) {
    return new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest()

      // Detect if this is a direct MinIO upload (for better error handling)
      const isDirect = presignedURL.includes('192.168.1.127:9000')
      const uploadType = isDirect ? 'direct MinIO' : 'CloudFlare CDN'

      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) {
          const progress = (e.loaded / e.total) * 100
          onProgress(progress)
        }
      }

      xhr.onload = () => {
        if (xhr.status === 200 || xhr.status === 204) {
          console.log(`✅ Upload successful via ${uploadType}`)
          resolve()
        } else {
          const errorMsg = `Upload failed via ${uploadType} with status ${xhr.status}`
          console.error(errorMsg)
          reject(new Error(errorMsg))
        }
      }

      xhr.onerror = (e) => {
        const errorMsg = `Upload failed via ${uploadType}: Network error`
        console.error(errorMsg, e)
        
        // Provide specific guidance for direct MinIO uploads
        if (isDirect) {
          reject(new Error(`${errorMsg}. Please ensure MinIO server is accessible and CORS is properly configured.`))
        } else {
          reject(new Error(errorMsg))
        }
      }

      xhr.ontimeout = () => {
        const errorMsg = `Upload timed out via ${uploadType}`
        console.error(errorMsg)
        reject(new Error(errorMsg))
      }

      // Set appropriate timeout (longer for large files via direct MinIO)
      xhr.timeout = isDirect ? 30 * 60 * 1000 : 10 * 60 * 1000 // 30min for direct, 10min for CloudFlare

      xhr.open('PUT', presignedURL)
      // Preserve original WAV quality - no compression headers
      xhr.setRequestHeader('Content-Type', 'audio/wav')
      
      console.log(`🚀 Starting upload via ${uploadType} (${(file.size / 1024 / 1024).toFixed(1)} MB)`)
      xhr.send(file) // Send raw file data - no compression
    })
  },

  // Mark upload as complete for processing
  async completeUpload(filename: string) {
    const response = await fetch(`${API_BASE}/api/upload/complete`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ filename })
    })

    if (!response.ok) {
      throw new Error('Failed to process uploaded file')
    }

    return response.json()
  },

  // Mark batch upload as complete for processing (triggers Discord batch notifications)
  async completeUploadBatch(filenames: string[]) {
    const response = await fetch(`${API_BASE}/api/upload/complete-batch`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ filenames })
    })

    if (!response.ok) {
      throw new Error('Failed to process uploaded batch')
    }

    return response.json()
  },

  // List files
  async listFiles() {
    const response = await fetch(`${API_BASE}/api/files`)
    if (!response.ok) {
      throw new Error('Failed to fetch files')
    }
    return response.json()
  },

  // Test Discord webhook
  async testDiscord() {
    const response = await fetch(`${API_BASE}/api/test/discord`, {
      method: 'POST'
    })
    
    if (!response.ok) {
      throw new Error('Discord test failed')
    }
    
    return response.json()
  },

  // Test MinIO connection
  async testMinIO() {
    const response = await fetch(`${API_BASE}/api/test/minio`)
    
    if (!response.ok) {
      throw new Error('MinIO test failed')
    }
    
    return response.json()
  },

  // Fast duplicate check (O(1) operation - works with millions of files)
  async checkDuplicate(filename: string) {
    const response = await fetch(`${API_BASE}/api/check-duplicate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ filename })
    })

    if (!response.ok) {
      throw new Error('Duplicate check failed')
    }

    const result = await response.json()
    return result.isDuplicate
  }
}

// WebSocket connection
export const createWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws`
  return new WebSocket(wsUrl)
}