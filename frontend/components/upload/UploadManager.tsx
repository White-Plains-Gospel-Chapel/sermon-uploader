import React from 'react'
import { UploadDropzone } from './UploadDropzone'
import { UploadQueue } from './UploadQueue'
import { UploadStats } from './UploadStats'
import { useUploadQueue } from '@/hooks/useUploadQueue'
import { Loader2 } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'

export function UploadManager() {
  const { 
    files, 
    isProcessing, 
    addFiles, 
    removeFile, 
    clearCompleted,
    stats 
  } = useUploadQueue()

  return (
    <div className="space-y-6">
      <UploadDropzone 
        onFilesSelected={addFiles}
        disabled={isProcessing}
      />
      
      {isProcessing && (
        <Card className="border-primary/20 bg-primary/5">
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <Loader2 className="h-5 w-5 text-primary animate-spin" />
              <div>
                <p className="font-medium text-primary">Processing uploads...</p>
                <p className="text-sm text-slate-600">
                  {stats.completed} of {stats.total} files completed
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
      
      {stats.total > 0 && (
        <UploadStats 
          stats={stats}
          onClearCompleted={clearCompleted}
        />
      )}
      
      <UploadQueue 
        files={files}
        onRemoveFile={removeFile}
      />
    </div>
  )
}