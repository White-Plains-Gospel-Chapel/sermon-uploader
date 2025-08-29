import { useState, useRef, useCallback, useEffect } from 'react'
import { UploadFile, UploadQueueState } from '@/types'
import { generateFileId, filterValidFiles } from '@/utils/fileHelpers'
import { ConcurrencyLimiter, getOptimalConcurrency, delay } from '@/utils/concurrency'
import { chunkUploadService, ChunkUploadProgress } from '@/services/chunkUploadService'
import { CHUNK_SIZE } from '@/utils/chunking'

interface ChunkedUploadFile extends UploadFile {
  totalChunks?: number
  completedChunks?: number
  chunkProgress?: number
  canResume?: boolean
}

interface UploadStats {
  totalBytes: number
  uploadedBytes: number
  totalChunks: number
  completedChunks: number
  startTime: number
  currentSpeed: number
}

export function useChunkedUploadQueue() {
  const [state, setState] = useState<UploadQueueState>({
    files: [] as ChunkedUploadFile[],
    isProcessing: false
  })
  
  const [uploadStats, setUploadStats] = useState<UploadStats>({
    totalBytes: 0,
    uploadedBytes: 0,
    totalChunks: 0,
    completedChunks: 0,
    startTime: 0,
    currentSpeed: 0
  })
  
  const queueRef = useRef<ChunkedUploadFile[]>([])
  const limiterRef = useRef<ConcurrencyLimiter>()
  
  // Initialize with lower concurrency for chunked uploads (more API calls)
  useEffect(() => {
    const baseConcurrency = getOptimalConcurrency()
    // Reduce concurrency for chunked uploads since each file makes multiple API calls
    const chunkConcurrency = Math.max(1, Math.floor(baseConcurrency / 2))
    limiterRef.current = new ConcurrencyLimiter(chunkConcurrency)
    console.log(`Chunked upload concurrency set to ${chunkConcurrency} (base: ${baseConcurrency})`)
  }, [])

  const addFiles = useCallback((newFiles: File[]) => {
    const validFiles = filterValidFiles(newFiles)
    
    const uploadFiles: ChunkedUploadFile[] = validFiles.map(file => {
      const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
      
      return {
        file,
        id: generateFileId(),
        status: 'queued' as const,
        progress: 0,
        totalChunks,
        completedChunks: 0,
        chunkProgress: 0,
        canResume: false
      }
    })
    
    setState(prev => ({
      ...prev,
      files: [...prev.files, ...uploadFiles]
    }))
    
    queueRef.current.push(...uploadFiles)
    processQueue()
  }, [])

  const updateFile = useCallback((id: string, updates: Partial<ChunkedUploadFile>) => {
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
    
    // Calculate stats
    const totalBytes = filesToProcess.reduce((sum, f) => sum + f.file.size, 0)
    const totalChunks = filesToProcess.reduce((sum, f) => sum + (f.totalChunks || 0), 0)
    
    setUploadStats({
      totalBytes,
      uploadedBytes: 0,
      totalChunks,
      completedChunks: 0,
      startTime: Date.now(),
      currentSpeed: 0
    })
    
    // Process files in parallel with limited concurrency
    const uploadPromises = filesToProcess.map((uploadFile, index) =>
      limiterRef.current!.run(async () => {
        // Stagger starts to prevent API overload
        await delay(index * 200)
        await uploadChunkedFile(uploadFile)
      })
    )
    
    await Promise.allSettled(uploadPromises)
    setState(prev => ({ ...prev, isProcessing: false }))
  }, [state.isProcessing])

  const uploadChunkedFile = async (uploadFile: ChunkedUploadFile) => {
    const startTime = Date.now()
    let lastUpdateTime = startTime
    let lastCompletedChunks = 0
    
    try {
      updateFile(uploadFile.id, { 
        status: 'checking',
        chunkProgress: 0,
        completedChunks: 0 
      })
      
      updateFile(uploadFile.id, { status: 'uploading' })
      
      await chunkUploadService.uploadFileInChunks(
        uploadFile.file,
        // Progress callback
        (progress: ChunkUploadProgress) => {
          updateFile(uploadFile.id, {
            progress: progress.overallProgress,
            chunkProgress: progress.chunkProgress
          })
        },
        // Chunk complete callback
        (chunkIndex: number, totalChunks: number) => {
          const completedChunks = chunkIndex + 1
          updateFile(uploadFile.id, {
            completedChunks
          })
          
          // Update global stats
          const now = Date.now()
          if (now - lastUpdateTime > 1000) { // Update every second
            const newChunks = completedChunks - lastCompletedChunks
            const timeDiff = (now - lastUpdateTime) / 1000
            const chunkSpeed = newChunks / timeDiff // chunks per second
            
            setUploadStats(prev => ({
              ...prev,
              completedChunks: prev.completedChunks + newChunks,
              currentSpeed: chunkSpeed * CHUNK_SIZE // Convert to bytes per second
            }))
            
            lastUpdateTime = now
            lastCompletedChunks = completedChunks
          }
        }
      )
      
      updateFile(uploadFile.id, { 
        status: 'success', 
        progress: 100,
        completedChunks: uploadFile.totalChunks 
      })
      
      // Update final stats
      setUploadStats(prev => ({
        ...prev,
        uploadedBytes: prev.uploadedBytes + uploadFile.file.size
      }))
      
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Upload failed'
      
      if (errorMessage === 'File already exists') {
        updateFile(uploadFile.id, {
          status: 'duplicate',
          progress: 100,
          error: 'File already exists'
        })
      } else {
        updateFile(uploadFile.id, {
          status: 'error',
          error: errorMessage,
          canResume: true // Can potentially resume chunked uploads
        })
      }
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

  const retryFile = useCallback(async (fileId: string) => {
    const file = state.files.find(f => f.id === fileId) as ChunkedUploadFile
    if (!file || file.status !== 'error') return
    
    updateFile(fileId, { status: 'queued', error: undefined })
    queueRef.current.push(file)
    
    if (!state.isProcessing) {
      processQueue()
    }
  }, [state.files, state.isProcessing, processQueue])

  // Format stats for display
  const formatSpeed = (bytesPerSecond: number): string => {
    if (bytesPerSecond < 1024) return `${Math.round(bytesPerSecond)} B/s`
    if (bytesPerSecond < 1024 * 1024) return `${Math.round(bytesPerSecond / 1024)} KB/s`
    return `${(bytesPerSecond / (1024 * 1024)).toFixed(1)} MB/s`
  }

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
    retryFile,
    stats: {
      total: state.files.length,
      completed: state.files.filter(f => f.status === 'success').length,
      failed: state.files.filter(f => f.status === 'error').length,
      duplicates: state.files.filter(f => f.status === 'duplicate').length,
      uploading: state.files.filter(f => f.status === 'uploading').length
    },
    chunkStats: {
      totalChunks: uploadStats.totalChunks,
      completedChunks: uploadStats.completedChunks,
      chunkProgress: uploadStats.totalChunks > 0 
        ? Math.round((uploadStats.completedChunks / uploadStats.totalChunks) * 100) 
        : 0
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