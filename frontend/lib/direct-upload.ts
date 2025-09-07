// Direct MinIO upload service - bypasses CloudFlare completely
// Creates a peer-to-peer connection between browser and MinIO bucket

const BACKEND_URL = 'http://192.168.1.127:8000' // Direct to Pi, no CloudFlare

export interface DirectUploadOptions {
  onProgress?: (progress: number) => void
  onComplete?: () => void
  onError?: (error: Error) => void
}

// Get direct MinIO presigned URL (bypasses CloudFlare)
export async function getDirectMinIOUrl(filename: string, fileSize: number) {
  console.log(`🎯 Getting direct MinIO URL for ${filename} (${(fileSize / 1024 / 1024).toFixed(1)} MB)`)
  
  const response = await fetch(`${BACKEND_URL}/api/upload/direct-minio`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ filename, fileSize })
  })

  const result = await response.json()

  if (!response.ok) {
    if (response.status === 409 && result.isDuplicate) {
      const error = new Error('File already exists') as Error & { isDuplicate: boolean }
      error.isDuplicate = true
      throw error
    }
    throw new Error(result.message || 'Failed to get direct upload URL')
  }

  console.log(`✅ Direct MinIO URL obtained - no CloudFlare in path!`)
  return result
}

// Upload directly to MinIO (peer-to-peer, no CloudFlare)
export async function uploadDirectToMinIO(
  file: File,
  presignedURL: string,
  options: DirectUploadOptions = {}
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    
    console.log(`🚀 Starting direct MinIO upload for ${file.name}`)
    console.log(`📡 Peer-to-peer connection - bypassing CloudFlare completely`)
    
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const progress = (e.loaded / e.total) * 100
        options.onProgress?.(progress)
      }
    }
    
    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 204) {
        console.log(`✅ Direct upload successful - no proxy, no CloudFlare!`)
        options.onComplete?.()
        resolve()
      } else {
        const error = new Error(`Direct upload failed with status ${xhr.status}`)
        console.error(error)
        options.onError?.(error)
        reject(error)
      }
    }
    
    xhr.onerror = () => {
      const error = new Error(`Network error during direct MinIO upload`)
      console.error(error)
      options.onError?.(error)
      reject(error)
    }
    
    xhr.ontimeout = () => {
      const error = new Error(`Direct upload timed out`)
      console.error(error)
      options.onError?.(error)
      reject(error)
    }
    
    // Set long timeout for large files
    xhr.timeout = 60 * 60 * 1000 // 1 hour for very large files
    
    // Direct PUT to MinIO presigned URL
    xhr.open('PUT', presignedURL)
    
    // MinIO expects the content type
    xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream')
    
    // Send the file directly to MinIO
    xhr.send(file)
  })
}

// Batch upload directly to MinIO
export async function getDirectMinIOUrlBatch(files: Array<{filename: string, fileSize: number}>) {
  console.log(`🎯 Getting direct MinIO URLs for ${files.length} files`)
  
  const response = await fetch(`${BACKEND_URL}/api/upload/direct-minio-batch`, {
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

  console.log(`✅ Got ${result.success_count} direct MinIO URLs`)
  return result
}

// Complete upload (notify backend that upload is done)
export async function completeDirectUpload(filename: string) {
  const response = await fetch(`${BACKEND_URL}/api/upload/complete`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ filename })
  })

  if (!response.ok) {
    throw new Error('Failed to complete upload')
  }

  return response.json()
}