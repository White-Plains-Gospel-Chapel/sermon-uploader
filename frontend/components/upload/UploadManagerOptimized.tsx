import React from 'react'
import { UploadDropzone } from './UploadDropzone'
import { UploadQueue } from './UploadQueue'
import { UploadStats } from './UploadStats'
import { useUploadQueueOptimized } from '@/hooks/useUploadQueueOptimized'
import { Loader2, Zap, Clock, Gauge } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'

export function UploadManagerOptimized() {
  const { 
    files, 
    isProcessing, 
    addFiles, 
    removeFile, 
    clearCompleted,
    stats,
    performance
  } = useUploadQueueOptimized()

  return (
    <div className="space-y-6">
      <UploadDropzone 
        onFilesSelected={addFiles}
        disabled={isProcessing}
      />
      
      {isProcessing && (
        <div className="space-y-4">
          {/* Performance Metrics Card */}
          <Card className="border-primary/20 bg-gradient-to-r from-primary/5 to-primary/10">
            <CardContent className="p-4">
              <div className="space-y-4">
                {/* Main Processing Status */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Loader2 className="h-5 w-5 text-primary animate-spin" />
                    <div>
                      <p className="font-medium text-primary">
                        Processing {stats.uploading} files in parallel
                      </p>
                      <p className="text-sm text-slate-600">
                        {stats.completed} of {stats.total} completed
                      </p>
                    </div>
                  </div>
                  <Badge className="bg-primary/10 text-primary border-primary/20">
                    {performance.concurrency} concurrent
                  </Badge>
                </div>
                
                {/* Overall Progress Bar */}
                <div className="space-y-2">
                  <Progress value={performance.progress} className="h-2" />
                  <div className="flex justify-between text-xs text-slate-600">
                    <span>{performance.progress}% complete</span>
                    <span>{performance.timeRemaining} remaining</span>
                  </div>
                </div>
                
                {/* Performance Metrics */}
                <div className="grid grid-cols-3 gap-4 pt-2 border-t border-slate-200">
                  <div className="flex items-center gap-2">
                    <Gauge className="h-4 w-4 text-blue-600" />
                    <div>
                      <p className="text-xs text-slate-500">Upload Speed</p>
                      <p className="text-sm font-medium text-slate-900">
                        {performance.speed}
                      </p>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2">
                    <Zap className="h-4 w-4 text-yellow-600" />
                    <div>
                      <p className="text-xs text-slate-500">Parallel Uploads</p>
                      <p className="text-sm font-medium text-slate-900">
                        {performance.concurrency} active
                      </p>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2">
                    <Clock className="h-4 w-4 text-green-600" />
                    <div>
                      <p className="text-xs text-slate-500">Time Remaining</p>
                      <p className="text-sm font-medium text-slate-900">
                        {performance.timeRemaining}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
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