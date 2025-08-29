import React from 'react'
import { FileRow } from './FileRow'
import { UploadFile } from '@/types'
import { Badge } from '@/components/ui/badge'

interface UploadQueueProps {
  files: UploadFile[]
  onRemoveFile: (id: string) => void
}

export function UploadQueue({ files, onRemoveFile }: UploadQueueProps) {
  if (files.length === 0) {
    return null
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-slate-900">Upload Progress</h2>
        <Badge variant="secondary" className="text-sm">
          {files.length} {files.length === 1 ? 'file' : 'files'}
        </Badge>
      </div>
      
      <div className="bg-white rounded-lg border border-slate-200 overflow-hidden">
        {/* Table Header */}
        <div className="bg-slate-50 px-6 py-3 border-b border-slate-200">
          <div className="grid grid-cols-12 gap-4 text-sm font-medium text-slate-600">
            <div className="col-span-6">Name</div>
            <div className="col-span-2">Status</div>
            <div className="col-span-2">Size</div>
            <div className="col-span-2">Progress</div>
          </div>
        </div>
        
        {/* File Rows */}
        <div className="divide-y divide-slate-100">
          {files.map(file => (
            <FileRow
              key={file.id}
              file={file}
              onRemove={onRemoveFile}
            />
          ))}
        </div>
      </div>
    </div>
  )
}