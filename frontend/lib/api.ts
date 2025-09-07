// API configuration
const getApiUrl = () => {
  // DUAL-DOMAIN ARCHITECTURE: Web app through CloudFlare, MinIO direct
  // This allows global access while bypassing CloudFlare for uploads
  if (typeof window !== 'undefined') {
    // Use CloudFlare for web app API calls (better performance, protection)
    const cloudflareUrl = process.env.NEXT_PUBLIC_CLOUDFLARE_URL || `${window.location.protocol}//${window.location.host}`
    console.log('☁️ Using CloudFlare for API calls (web app)')
    console.log('🎯 MinIO uploads will bypass CloudFlare directly')
    return cloudflareUrl
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

  // Get presigned URL for upload (ZERO-MEMORY STREAMING - no memory usage)
  async getPresignedURL(filename: string, fileSize: number) {
    // Use zero-memory streaming proxy to handle bulk uploads without freezing
    const endpoint = `${API_BASE}/api/upload/zero-memory-url`
    
    const response = await fetch(endpoint, {
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

  // Get presigned URLs for multiple files at once (ZERO-MEMORY STREAMING)
  async getPresignedURLsBatch(files: Array<{filename: string, fileSize: number}>) {
    // Use batch endpoint for zero-memory streaming URLs
    const response = await fetch(`${API_BASE}/api/upload/zero-memory-url-batch`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ files })
    })
    
    const result = await response.json()
    
    if (!response.ok) {
      throw new Error(result.message || 'Failed to get batch upload URLs')
    }
    
    console.log(`🎯 Got ${result.success_count} zero-memory streaming URLs (no memory usage)`)
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

  // Upload to MinIO using presigned URL or proxy endpoint
  async uploadToMinIO(file: File, presignedURL: string, uploadMethod?: string, onProgress?: (progress: number) => void) {
    // Add small delay for bulk uploads to prevent browser freeze
    const isZeroMemory = presignedURL.includes('zero-memory')
    if (isZeroMemory) {
      await new Promise(resolve => setTimeout(resolve, 100)) // 100ms delay
    }
    return new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest()

      // Better detection with logging
      const isProxy = presignedURL.includes('/api/upload/proxy') || uploadMethod === 'backend_proxy'
      const isZeroMemory = presignedURL.includes('zero-memory') || uploadMethod === 'zero_memory_streaming'
      const isStreaming = presignedURL.includes('streaming-proxy') || uploadMethod === 'streaming_proxy'
      const isDirect = presignedURL.includes('minio.') || presignedURL.includes(':9000') || uploadMethod === 'direct_minio'
      const uploadType = isZeroMemory ? 'Zero-Memory Streaming (bulk safe)' : (isStreaming ? 'Streaming Proxy (no CORS)' : (isDirect ? 'Direct MinIO' : (isProxy ? 'Backend proxy' : 'CloudFlare CDN')))
      
      console.log('🔍 Upload Detection:', {
        url: presignedURL,
        method: uploadMethod,
        isProxy,
        isDirect,
        uploadType,
        fileSize: `${(file.size / 1024 / 1024).toFixed(1)} MB`
      })

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

      // Set appropriate timeout (longer for large files)
      const timeoutMinutes = Math.max(5, Math.ceil(file.size / (50 * 1024 * 1024))) // 1 min per 50MB, min 5 min
      xhr.timeout = timeoutMinutes * 60 * 1000

      // Use appropriate HTTP method based on upload type
      if (isProxy) {
        // For proxy uploads, we need to POST the file
        xhr.open('PUT', presignedURL)
        xhr.setRequestHeader('Content-Type', 'audio/wav')
      } else {
        // For presigned URLs, use PUT
        xhr.open('PUT', presignedURL)
        // Preserve original WAV quality - no compression headers
        xhr.setRequestHeader('Content-Type', 'audio/wav')
      }
      
      console.log(`🚀 Starting ${uploadType} upload: ${file.name} (${(file.size / 1024 / 1024).toFixed(1)} MB)`)
      
      // Add progress logging
      let lastLoggedPercent = 0
      xhr.upload.onprogress = (e) => {
        if (onProgress) {
          const progress = (e.loaded / e.total) * 100
          onProgress(progress)
        }
        
        if (e.lengthComputable) {
          const percent = Math.floor((e.loaded / e.total) * 100)
          if (percent > lastLoggedPercent && percent % 10 === 0) {
            console.log(`📊 ${file.name}: ${percent}% uploaded`)
            lastLoggedPercent = percent
          }
        }
      }
      
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
  // Use direct Pi connection for WebSocket too
  const bypassCloudFlare = true
  
  if (bypassCloudFlare) {
    // Direct WebSocket to Pi
    const directPiWs = process.env.NEXT_PUBLIC_DIRECT_PI_WS || 'ws://192.168.1.127:8000/ws'
    console.log('🎯 WebSocket using direct Pi connection')
    return new WebSocket(directPiWs)
  } else {
    // Through CloudFlare
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`
    return new WebSocket(wsUrl)
  }
}