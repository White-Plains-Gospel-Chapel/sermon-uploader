import React from 'react'
import { X, FileAudio, CheckCircle, AlertCircle, Clock, Loader2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui/button'
import { UploadFile } from '@/types'
import { formatFileSize } from '@/utils/fileHelpers'
import { UI_TEXT } from '@/utils/constants'

interface FileRowProps {
  file: UploadFile
  onRemove: (id: string) => void
}

export function FileRow({ file, onRemove }: FileRowProps) {
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
            {UI_TEXT.STATUS.UPLOADING}
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
            {UI_TEXT.STATUS.ERROR}
          </Badge>
        )
    }
  }

  const canRemove = file.status !== 'uploading' && file.status !== 'checking'

  return (
    <div className="px-6 py-4 grid grid-cols-12 gap-4 items-center hover:bg-slate-50 transition-colors">
      {/* File Name & Icon */}
      <div className="col-span-6 flex items-center gap-3 min-w-0">
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
      </div>
      
      {/* Progress / Actions */}
      <div className="col-span-2 flex items-center gap-2">
        {file.status === 'uploading' ? (
          <div className="flex-1">
            <Progress value={file.progress} className="h-2" />
            <span className="text-xs text-slate-500 mt-1">{file.progress}%</span>
          </div>
        ) : (
          canRemove && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onRemove(file.id)}
              className="h-8 w-8 p-0 hover:bg-red-50 hover:text-red-600"
            >
              <X className="h-4 w-4" />
            </Button>
          )
        )}
      </div>
    </div>
  )
}