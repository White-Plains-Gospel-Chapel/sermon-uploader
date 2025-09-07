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
    
    // Track successful uploads for batch completion
    const successfulUploads: string[] = []
    
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
      
      // Upload the file but don't call individual completion yet
      const success = await uploadSingleFileForBatch(uploadFile, result.uploadUrl, result.uploadMethod)
      if (success) {
        successfulUploads.push(uploadFile.file.name)
      }
    }
    
    // Call batch completion endpoint to trigger Discord batch notifications
    if (successfulUploads.length > 0) {
      try {
        await uploadService.completeUploadBatch(successfulUploads)
        console.log(`✅ Batch completion processed for ${successfulUploads.length} files`)
      } catch (error) {
        console.warn(`Batch completion failed, falling back to individual processing:`, error)
        // Fallback: process individual completions
        for (const filename of successfulUploads) {
          uploadService.completeUpload(filename).catch(err => {
            console.warn(`Individual completion fallback failed for ${filename}:`, err)
          })
        }
      }
    }
  }

  const processIndividual = async (files: UploadFile[]) => {
    for (const uploadFile of files) {
      try {
        updateFile(uploadFile.id, { status: 'checking' })
        
        const presignedResponse = await uploadService.getPresignedURL(
          uploadFile.file.name,
          uploadFile.file.size
        )
        
        if (presignedResponse.isDuplicate) {
          updateFile(uploadFile.id, {
            status: 'duplicate',
            progress: 100,
            error: 'File already exists'
          })
          continue
        }
        
        await uploadSingleFile(uploadFile, presignedResponse.uploadUrl, presignedResponse.uploadMethod)
      } catch (error) {
        updateFile(uploadFile.id, {
          status: 'error',
          error: error instanceof Error ? error.message : 'Upload failed'
        })
      }
    }
  }

  const uploadSingleFile = async (uploadFile: UploadFile, uploadUrl: string, uploadMethod?: string) => {
    try {
      updateFile(uploadFile.id, { status: 'uploading', progress: 0 })
      
      await uploadService.uploadToMinIO(
        uploadFile.file,
        uploadUrl,
        uploadMethod,
        (progress) => updateFile(uploadFile.id, { progress: Math.round(progress) })
      )
      
      updateFile(uploadFile.id, { status: 'success', progress: 100 })
      
      // Process metadata asynchronously (for individual uploads only)
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

  const uploadSingleFileForBatch = async (uploadFile: UploadFile, uploadUrl: string, uploadMethod?: string): Promise<boolean> => {
    try {
      updateFile(uploadFile.id, { status: 'uploading', progress: 0 })
      
      await uploadService.uploadToMinIO(
        uploadFile.file,
        uploadUrl,
        uploadMethod,
        (progress) => updateFile(uploadFile.id, { progress: Math.round(progress) })
      )
      
      updateFile(uploadFile.id, { status: 'success', progress: 100 })
      
      // Don't call individual completion - this will be handled by batch completion
      return true
    } catch (error) {
      updateFile(uploadFile.id, {
        status: 'error',
        error: error instanceof Error ? error.message : 'Upload failed'
      })
      return false
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