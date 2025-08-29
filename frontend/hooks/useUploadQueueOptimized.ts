import { useState, useRef, useCallback, useEffect } from 'react'
import { UploadFile, UploadQueueState } from '@/types'
import { generateFileId, filterValidFiles } from '@/utils/fileHelpers'
import { ConcurrencyLimiter, getOptimalConcurrency, delay } from '@/utils/concurrency'
import { uploadService } from '@/services/uploadService'

interface UploadStats {
  totalBytes: number
  uploadedBytes: number
  startTime: number
  currentSpeed: number // bytes per second
}

export function useUploadQueueOptimized() {
  const [state, setState] = useState<UploadQueueState>({
    files: [],
    isProcessing: false
  })
  
  const [uploadStats, setUploadStats] = useState<UploadStats>({
    totalBytes: 0,
    uploadedBytes: 0,
    startTime: 0,
    currentSpeed: 0
  })
  
  const queueRef = useRef<UploadFile[]>([])
  const limiterRef = useRef<ConcurrencyLimiter>()
  
  // Initialize concurrency limiter based on device capabilities
  useEffect(() => {
    const concurrency = getOptimalConcurrency()
    limiterRef.current = new ConcurrencyLimiter(concurrency)
    console.log(`Upload concurrency set to ${concurrency} based on device capabilities`)
  }, [])

  const addFiles = useCallback((newFiles: File[]) => {
    const validFiles = filterValidFiles(newFiles)
    
    const uploadFiles: UploadFile[] = validFiles.map(file => ({
      file,
      id: generateFileId(),
      status: 'queued' as const,
      progress: 0
    }))
    
    setState(prev => ({
      ...prev,
      files: [...prev.files, ...uploadFiles]
    }))
    
    queueRef.current.push(...uploadFiles)
    processQueue()
  }, [])

  const updateFile = useCallback((id: string, updates: Partial<UploadFile>) => {
    setState(prev => ({
      ...prev,
      files: prev.files.map(f => 
        f.id === id ? { ...f, ...updates } : f
      )
    }))
  }, [])

  const removeFile = useCallback((id: string) => {
    setState(prev => ({
      ...prev,
      files: prev.files.filter(f => f.id !== id)
    }))
    queueRef.current = queueRef.current.filter(f => f.id !== id)
  }, [])

  const processQueue = useCallback(async () => {
    if (state.isProcessing || queueRef.current.length === 0) return
    if (!limiterRef.current) return
    
    setState(prev => ({ ...prev, isProcessing: true }))
    
    const filesToProcess = [...queueRef.current]
    queueRef.current = []
    
    // Calculate total bytes for progress tracking
    const totalBytes = filesToProcess.reduce((sum, f) => sum + f.file.size, 0)
    setUploadStats({
      totalBytes,
      uploadedBytes: 0,
      startTime: Date.now(),
      currentSpeed: 0
    })
    
    try {
      // Get all presigned URLs first (batch request)
      await processBatchParallel(filesToProcess)
    } catch (error) {
      console.error('Batch processing failed, falling back to individual uploads:', error)
      await processIndividualParallel(filesToProcess)
    }
    
    setState(prev => ({ ...prev, isProcessing: false }))
  }, [state.isProcessing])

  const processBatchParallel = async (files: UploadFile[]) => {
    // Mark all as checking
    files.forEach(({ id }) => updateFile(id, { status: 'checking' }))
    
    const batchRequest = files.map(f => ({
      filename: f.file.name,
      fileSize: f.file.size
    }))
    
    const batchResponse = await uploadService.getPresignedURLsBatch(batchRequest)
    
    // Process files in parallel with concurrency limit
    const uploadPromises = files.map((uploadFile, index) => 
      limiterRef.current!.run(async () => {
        // Add small delay between starts to prevent thundering herd
        await delay(index * 100)
        
        const result = batchResponse.results[uploadFile.file.name]
        
        if (result.error) {
          updateFile(uploadFile.id, {
            status: 'error',
            error: result.message || 'Upload failed'
          })
          return
        }
        
        if (result.isDuplicate) {
          updateFile(uploadFile.id, {
            status: 'duplicate',
            progress: 100,
            error: 'File already exists'
          })
          return
        }
        
        await uploadSingleFile(uploadFile, result.uploadUrl)
      })
    )
    
    // Wait for all uploads to complete
    await Promise.allSettled(uploadPromises)
  }

  const processIndividualParallel = async (files: UploadFile[]) => {
    const uploadPromises = files.map((uploadFile, index) =>
      limiterRef.current!.run(async () => {
        // Add small delay between starts
        await delay(index * 100)
        
        try {
          updateFile(uploadFile.id, { status: 'checking' })
          
          const { uploadUrl, isDuplicate } = await uploadService.getPresignedURL(
            uploadFile.file.name,
            uploadFile.file.size
          )
          
          if (isDuplicate) {
            updateFile(uploadFile.id, {
              status: 'duplicate',
              progress: 100,
              error: 'File already exists'
            })
            return
          }
          
          await uploadSingleFile(uploadFile, uploadUrl)
        } catch (error) {
          updateFile(uploadFile.id, {
            status: 'error',
            error: error instanceof Error ? error.message : 'Upload failed'
          })
        }
      })
    )
    
    await Promise.allSettled(uploadPromises)
  }

  const uploadSingleFile = async (uploadFile: UploadFile, uploadUrl: string) => {
    const startTime = Date.now()
    let lastProgressTime = startTime
    let lastProgressBytes = 0
    
    try {
      updateFile(uploadFile.id, { status: 'uploading', progress: 0 })
      
      await uploadService.uploadToMinIO(
        uploadFile.file,
        uploadUrl,
        (progress) => {
          const now = Date.now()
          const currentBytes = (uploadFile.file.size * progress) / 100
          
          // Update individual file progress
          updateFile(uploadFile.id, { progress: Math.round(progress) })
          
          // Calculate upload speed every 500ms
          if (now - lastProgressTime > 500) {
            const timeDiff = (now - lastProgressTime) / 1000 // seconds
            const bytesDiff = currentBytes - lastProgressBytes
            const speed = bytesDiff / timeDiff // bytes per second
            
            setUploadStats(prev => ({
              ...prev,
              uploadedBytes: prev.uploadedBytes + bytesDiff,
              currentSpeed: speed
            }))
            
            lastProgressTime = now
            lastProgressBytes = currentBytes
          }
        }
      )
      
      updateFile(uploadFile.id, { status: 'success', progress: 100 })
      
      // Update final stats
      setUploadStats(prev => ({
        ...prev,
        uploadedBytes: prev.uploadedBytes + uploadFile.file.size
      }))
      
      // Process metadata asynchronously
      uploadService.completeUpload(uploadFile.file.name).catch(error => {
        console.warn(`Metadata processing failed for ${uploadFile.file.name}:`, error)
      })
    } catch (error) {
      updateFile(uploadFile.id, {
        status: 'error',
        error: error instanceof Error ? error.message : 'Upload failed'
      })
    }
  }

  const clearCompleted = useCallback(() => {
    setState(prev => ({
      ...prev,
      files: prev.files.filter(f => 
        f.status !== 'success' && f.status !== 'duplicate' && f.status !== 'error'
      )
    }))
  }, [])

  // Format upload speed for display
  const formatSpeed = (bytesPerSecond: number): string => {
    if (bytesPerSecond < 1024) return `${Math.round(bytesPerSecond)} B/s`
    if (bytesPerSecond < 1024 * 1024) return `${Math.round(bytesPerSecond / 1024)} KB/s`
    return `${(bytesPerSecond / (1024 * 1024)).toFixed(1)} MB/s`
  }

  // Calculate estimated time remaining
  const getTimeRemaining = (): string => {
    if (uploadStats.currentSpeed === 0) return 'Calculating...'
    
    const remainingBytes = uploadStats.totalBytes - uploadStats.uploadedBytes
    const secondsRemaining = remainingBytes / uploadStats.currentSpeed
    
    if (secondsRemaining < 60) return `${Math.round(secondsRemaining)}s`
    if (secondsRemaining < 3600) return `${Math.round(secondsRemaining / 60)}m`
    return `${Math.round(secondsRemaining / 3600)}h ${Math.round((secondsRemaining % 3600) / 60)}m`
  }

  return {
    files: state.files,
    isProcessing: state.isProcessing,
    addFiles,
    removeFile,
    clearCompleted,
    stats: {
      total: state.files.length,
      completed: state.files.filter(f => f.status === 'success').length,
      failed: state.files.filter(f => f.status === 'error').length,
      duplicates: state.files.filter(f => f.status === 'duplicate').length,
      uploading: state.files.filter(f => f.status === 'uploading').length
    },
    performance: {
      speed: formatSpeed(uploadStats.currentSpeed),
      timeRemaining: getTimeRemaining(),
      progress: uploadStats.totalBytes > 0 
        ? Math.round((uploadStats.uploadedBytes / uploadStats.totalBytes) * 100)
        : 0,
      concurrency: limiterRef.current?.active || 0
    }
  }
}