import { useState, useRef, useCallback } from 'react'
import { UploadFile, UploadQueueState } from '@/types'
import { generateFileId, filterValidFiles } from '@/utils/fileHelpers'
import { uploadService } from '@/services/uploadService'

export function useUploadQueue() {
  const [state, setState] = useState<UploadQueueState>({
    files: [],
    isProcessing: false
  })
  
  const queueRef = useRef<UploadFile[]>([])

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
    
    setState(prev => ({ ...prev, isProcessing: true }))
    
    const filesToProcess = [...queueRef.current]
    queueRef.current = []
    
    try {
      // Try batch processing first
      await processBatch(filesToProcess)
    } catch (error) {
      console.error('Batch processing failed, falling back to individual uploads:', error)
      await processIndividual(filesToProcess)
    }
    
    setState(prev => ({ ...prev, isProcessing: false }))
  }, [state.isProcessing])

  const processBatch = async (files: UploadFile[]) => {
    // Mark all as checking
    files.forEach(({ id }) => updateFile(id, { status: 'checking' }))
    
    const batchRequest = files.map(f => ({
      filename: f.file.name,
      fileSize: f.file.size
    }))
    
    const batchResponse = await uploadService.getPresignedURLsBatch(batchRequest)
    
    // Process each file with its presigned URL
    for (const uploadFile of files) {
      const result = batchResponse.results[uploadFile.file.name]
      
      if (result.error) {
        updateFile(uploadFile.id, {
          status: 'error',
          error: result.message || 'Upload failed'
        })
        continue
      }
      
      if (result.isDuplicate) {
        updateFile(uploadFile.id, {
          status: 'duplicate',
          progress: 100,
          error: 'File already exists'
        })
        continue
      }
      
      await uploadSingleFile(uploadFile, result.uploadUrl)
    }
  }

  const processIndividual = async (files: UploadFile[]) => {
    for (const uploadFile of files) {
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
          continue
        }
        
        await uploadSingleFile(uploadFile, uploadUrl)
      } catch (error) {
        updateFile(uploadFile.id, {
          status: 'error',
          error: error instanceof Error ? error.message : 'Upload failed'
        })
      }
    }
  }

  const uploadSingleFile = async (uploadFile: UploadFile, uploadUrl: string) => {
    try {
      updateFile(uploadFile.id, { status: 'uploading', progress: 0 })
      
      await uploadService.uploadToMinIO(
        uploadFile.file,
        uploadUrl,
        (progress) => updateFile(uploadFile.id, { progress: Math.round(progress) })
      )
      
      updateFile(uploadFile.id, { status: 'success', progress: 100 })
      
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
      duplicates: state.files.filter(f => f.status === 'duplicate').length
    }
  }
}