// Chunked upload implementation to bypass CloudFlare's 100MB limit
const CHUNK_SIZE = 50 * 1024 * 1024 // 50MB chunks (well under CloudFlare's 100MB limit)

export interface ChunkUploadProgress {
  totalChunks: number
  currentChunk: number
  overallProgress: number
}

export async function uploadFileInChunks(
  file: File,
  uploadUrl: string,
  onProgress?: (progress: ChunkUploadProgress) => void
): Promise<void> {
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  
  console.log(`📦 Chunked upload: ${file.name} (${(file.size / 1024 / 1024).toFixed(1)} MB) in ${totalChunks} chunks`)
  
  for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
    const start = chunkIndex * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const chunk = file.slice(start, end)
    
    // Upload this chunk
    const chunkUrl = `${uploadUrl}&chunk=${chunkIndex}&totalChunks=${totalChunks}`
    
    await uploadChunk(chunk, chunkUrl, chunkIndex, totalChunks, (chunkProgress) => {
      const overallProgress = ((chunkIndex * 100) + chunkProgress) / totalChunks
      onProgress?.({
        totalChunks,
        currentChunk: chunkIndex + 1,
        overallProgress
      })
    })
  }
  
  // Notify backend that all chunks are uploaded
  await completeChunkedUpload(uploadUrl, totalChunks)
}

async function uploadChunk(
  chunk: Blob,
  url: string,
  chunkIndex: number,
  totalChunks: number,
  onProgress: (progress: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const progress = (e.loaded / e.total) * 100
        onProgress(progress)
      }
    }
    
    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 204) {
        console.log(`✅ Chunk ${chunkIndex + 1}/${totalChunks} uploaded`)
        resolve()
      } else {
        reject(new Error(`Chunk upload failed with status ${xhr.status}`))
      }
    }
    
    xhr.onerror = () => {
      reject(new Error(`Chunk ${chunkIndex + 1} upload failed: Network error`))
    }
    
    xhr.open('PUT', url)
    xhr.setRequestHeader('Content-Type', 'application/octet-stream')
    xhr.setRequestHeader('X-Chunk-Index', chunkIndex.toString())
    xhr.setRequestHeader('X-Total-Chunks', totalChunks.toString())
    
    xhr.send(chunk)
  })
}

async function completeChunkedUpload(baseUrl: string, totalChunks: number): Promise<void> {
  const completeUrl = baseUrl.replace('/upload/proxy', '/upload/complete-chunks')
  
  const response = await fetch(completeUrl, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ totalChunks })
  })
  
  if (!response.ok) {
    throw new Error('Failed to complete chunked upload')
  }
}