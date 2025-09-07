// Bulk Upload Manager - handles multiple uploads without overwhelming the system
export class BulkUploadManager {
  private uploadQueue: Array<{
    file: File
    uploadUrl: string
    uploadMethod: string
    resolve: (value: any) => void
    reject: (error: any) => void
  }> = []
  
  private isProcessing = false
  private maxConcurrent = 5
  private delayBetweenUploads = 100 // 100ms delay between uploads

  async addUpload(file: File, uploadUrl: string, uploadMethod: string): Promise<any> {
    return new Promise((resolve, reject) => {
      this.uploadQueue.push({
        file,
        uploadUrl,
        uploadMethod,
        resolve,
        reject
      })
      
      if (!this.isProcessing) {
        this.processQueue()
      }
    })
  }

  private async processQueue() {
    if (this.isProcessing || this.uploadQueue.length === 0) {
      return
    }

    this.isProcessing = true
    console.log(`📦 Processing bulk upload queue: ${this.uploadQueue.length} files`)

    const activeUploads = new Set<Promise<any>>()

    while (this.uploadQueue.length > 0 || activeUploads.size > 0) {
      // Start new uploads up to the concurrent limit
      while (activeUploads.size < this.maxConcurrent && this.uploadQueue.length > 0) {
        const uploadItem = this.uploadQueue.shift()!
        
        const uploadPromise = this.performUpload(uploadItem)
          .then(result => {
            activeUploads.delete(uploadPromise)
            uploadItem.resolve(result)
            return result
          })
          .catch(error => {
            activeUploads.delete(uploadPromise)
            uploadItem.reject(error)
            throw error
          })

        activeUploads.add(uploadPromise)

        // Add delay between starting uploads to prevent system overload
        if (this.uploadQueue.length > 0) {
          await this.delay(this.delayBetweenUploads)
        }
      }

      // Wait for at least one upload to complete
      if (activeUploads.size > 0) {
        try {
          await Promise.race(Array.from(activeUploads))
        } catch (error) {
          // Individual upload error - handled by the upload promise
        }
      }
    }

    this.isProcessing = false
    console.log(`✅ Bulk upload queue processing complete`)
  }

  private async performUpload(uploadItem: {
    file: File
    uploadUrl: string
    uploadMethod: string
  }): Promise<any> {
    const { file, uploadUrl, uploadMethod } = uploadItem

    console.log(`🚀 Starting zero-memory upload: ${file.name} (${(file.size / 1024 / 1024).toFixed(1)} MB)`)

    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()

      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const progress = (e.loaded / e.total) * 100
          // Could emit progress events here
        }
      }

      xhr.onload = () => {
        if (xhr.status === 200 || xhr.status === 204) {
          console.log(`✅ Zero-memory upload completed: ${file.name}`)
          resolve({
            success: true,
            filename: file.name,
            size: file.size,
            uploadMethod
          })
        } else {
          const error = new Error(`Upload failed with status ${xhr.status}`)
          console.error(`❌ Zero-memory upload failed: ${file.name}`, error)
          reject(error)
        }
      }

      xhr.onerror = () => {
        const error = new Error(`Network error during upload`)
        console.error(`❌ Network error for: ${file.name}`, error)
        reject(error)
      }

      xhr.ontimeout = () => {
        const error = new Error(`Upload timeout`)
        console.error(`❌ Timeout for: ${file.name}`, error)
        reject(error)
      }

      // Set timeout based on file size (larger files get more time)
      const timeoutMinutes = Math.max(5, Math.ceil(file.size / (10 * 1024 * 1024))) // 1 minute per 10MB, minimum 5 minutes
      xhr.timeout = timeoutMinutes * 60 * 1000

      xhr.open('PUT', uploadUrl)
      xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream')
      xhr.setRequestHeader('Content-Length', file.size.toString())
      xhr.send(file)
    })
  }

  private delay(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms))
  }

  getQueueStatus() {
    return {
      queued: this.uploadQueue.length,
      processing: this.isProcessing,
      maxConcurrent: this.maxConcurrent
    }
  }
}