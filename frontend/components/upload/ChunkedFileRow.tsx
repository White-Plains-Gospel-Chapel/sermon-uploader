import React from 'react'
import { X, FileAudio, CheckCircle, AlertCircle, Clock, Loader2, RotateCcw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui/button'
import { formatFileSize } from '@/utils/fileHelpers'
import { UI_TEXT } from '@/utils/constants'

interface ChunkedUploadFile {
  file: File
  id: string
  status: 'queued' | 'checking' | 'uploading' | 'success' | 'error' | 'duplicate'
  progress: number
  error?: string
  totalChunks?: number
  completedChunks?: number
  chunkProgress?: number
  canResume?: boolean
}

interface ChunkedFileRowProps {
  file: ChunkedUploadFile
  onRemove: (id: string) => void
  onRetry?: (id: string) => void
}

export function ChunkedFileRow({ file, onRemove, onRetry }: ChunkedFileRowProps) {
  const getStatusBadge = () => {
    switch (file.status) {
      case 'queued':
        return (
          <Badge variant="secondary" className="gap-1">
            <Clock className="h-3 w-3" />
            {UI_TEXT.STATUS.QUEUED}
          </Badge>
        )
      case 'checking':
        return (
          <Badge variant="secondary" className="gap-1">
            <Loader2 className="h-3 w-3 animate-spin" />
            {UI_TEXT.STATUS.CHECKING}
          </Badge>
        )
      case 'uploading':
        return (
          <Badge className="gap-1">
            <Loader2 className="h-3 w-3 animate-spin" />
            Uploading Chunks
          </Badge>
        )
      case 'success':
        return (
          <Badge variant="success" className="gap-1 bg-green-100 text-green-700">
            <CheckCircle className="h-3 w-3" />
            {UI_TEXT.STATUS.SUCCESS}
          </Badge>
        )
      case 'duplicate':
        return (
          <Badge variant="warning" className="gap-1 bg-yellow-100 text-yellow-700">
            <AlertCircle className="h-3 w-3" />
            {UI_TEXT.STATUS.DUPLICATE}
          </Badge>
        )
      case 'error':
        return (
          <Badge variant="destructive" className="gap-1">
            <AlertCircle className="h-3 w-3" />
            {file.canResume ? 'Can Resume' : 'Failed'}
          </Badge>
        )
    }
  }

  const canRemove = file.status !== 'uploading' && file.status !== 'checking'
  const canRetry = file.status === 'error' && file.canResume && onRetry

  const getChunkInfo = () => {
    if (!file.totalChunks) return null
    
    const completed = file.completedChunks || 0
    const total = file.totalChunks
    
    return (
      <div className="text-xs text-slate-500 mt-1">
        {completed}/{total} chunks • 4MB each
      </div>
    )
  }

  return (
    <div className="px-6 py-4 grid grid-cols-12 gap-4 items-center hover:bg-slate-50 transition-colors">
      {/* File Name & Icon */}
      <div className="col-span-5 flex items-center gap-3 min-w-0">
        <FileAudio className="h-5 w-5 text-blue-600 flex-shrink-0" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-slate-900 truncate" title={file.file.name}>
            {file.file.name}
          </p>
          {file.error && (
            <p className="text-xs text-red-600 mt-1 truncate" title={file.error}>
              {file.error}
            </p>
          )}
          {getChunkInfo()}
        </div>
      </div>
      
      {/* Status */}
      <div className="col-span-2">
        {getStatusBadge()}
      </div>
      
      {/* File Size */}
      <div className="col-span-2">
        <span className="text-sm text-slate-600">
          {formatFileSize(file.file.size)}
        </span>
        {file.totalChunks && (
          <div className="text-xs text-slate-500">
            {file.totalChunks} chunks
          </div>
        )}
      </div>
      
      {/* Progress / Actions */}
      <div className="col-span-3 flex items-center gap-2">
        {file.status === 'uploading' ? (
          <div className="flex-1">
            <Progress value={file.progress} className="h-2 mb-1" />
            <div className="flex justify-between text-xs text-slate-500">
              <span>{file.progress}% overall</span>
              {file.chunkProgress !== undefined && (
                <span>{Math.round(file.chunkProgress)}% chunk</span>
              )}
            </div>
            {file.completedChunks !== undefined && file.totalChunks && (
              <div className="text-xs text-slate-500 mt-1">
                {file.completedChunks}/{file.totalChunks} chunks done
              </div>
            )}
          </div>
        ) : (
          <div className="flex items-center gap-2">
            {canRetry && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onRetry(file.id)}
                className="h-8 w-8 p-0 hover:bg-blue-50 hover:text-blue-600"
                title="Resume upload"
              >
                <RotateCcw className="h-4 w-4" />
              </Button>
            )}
            {canRemove && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onRemove(file.id)}
                className="h-8 w-8 p-0 hover:bg-red-50 hover:text-red-600"
              >
                <X className="h-4 w-4" />
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}