import React from 'react'
import { UploadDropzone } from './UploadDropzone'
import { UploadQueue } from './UploadQueue'
import { UploadStats } from './UploadStats'
import { ChunkedFileRow } from './ChunkedFileRow'
import { useChunkedUploadQueue } from '@/hooks/useChunkedUploadQueue'
import { Loader2, Zap, Clock, Gauge, Package, HardDrive } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'

export function ChunkedUploadManager() {
  const { 
    files, 
    isProcessing, 
    addFiles, 
    removeFile, 
    clearCompleted,
    retryFile,
    stats,
    chunkStats,
    performance
  } = useChunkedUploadQueue()

  return (
    <div className="space-y-6">
      <UploadDropzone 
        onFilesSelected={addFiles}
        disabled={isProcessing}
      />
      
      {isProcessing && (
        <div className="space-y-4">
          {/* Enhanced Performance Metrics Card for Chunked Uploads */}
          <Card className="border-primary/20 bg-gradient-to-r from-primary/5 to-primary/10">
            <CardContent className="p-4">
              <div className="space-y-4">
                {/* Main Processing Status */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Loader2 className="h-5 w-5 text-primary animate-spin" />
                    <div>
                      <p className="font-medium text-primary">
                        Processing {stats.uploading} files with 4MB chunks
                      </p>
                      <p className="text-sm text-slate-600">
                        {stats.completed} of {stats.total} files completed
                      </p>
                    </div>
                  </div>
                  <Badge className="bg-primary/10 text-primary border-primary/20">
                    {performance.concurrency} parallel
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
                
                {/* Chunk Progress */}
                <div className="pt-2 border-t border-slate-200">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-slate-700">Chunk Progress</span>
                    <span className="text-sm text-slate-600">
                      {chunkStats.completedChunks}/{chunkStats.totalChunks} chunks
                    </span>
                  </div>
                  <Progress value={chunkStats.chunkProgress} className="h-1" />
                </div>
                
                {/* Performance Metrics */}
                <div className="grid grid-cols-4 gap-4 pt-2 border-t border-slate-200">
                  <div className="flex items-center gap-2">
                    <Gauge className="h-4 w-4 text-blue-600" />
                    <div>
                      <p className="text-xs text-slate-500">Speed</p>
                      <p className="text-sm font-medium text-slate-900">
                        {performance.speed}
                      </p>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2">
                    <Package className="h-4 w-4 text-green-600" />
                    <div>
                      <p className="text-xs text-slate-500">Chunks</p>
                      <p className="text-sm font-medium text-slate-900">
                        4MB each
                      </p>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2">
                    <Zap className="h-4 w-4 text-yellow-600" />
                    <div>
                      <p className="text-xs text-slate-500">Parallel</p>
                      <p className="text-sm font-medium text-slate-900">
                        {performance.concurrency} files
                      </p>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2">
                    <HardDrive className="h-4 w-4 text-purple-600" />
                    <div>
                      <p className="text-xs text-slate-500">Resume</p>
                      <p className="text-sm font-medium text-slate-900">
                        Enabled
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
      
      {/* Custom File Queue for Chunked Uploads */}
      {files.length > 0 && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold text-slate-900">
              Chunked Upload Progress
            </h2>
            <Badge variant="secondary" className="text-sm">
              {files.length} {files.length === 1 ? 'file' : 'files'}
            </Badge>
          </div>
          
          <div className="bg-white rounded-lg border border-slate-200 overflow-hidden">
            {/* Enhanced Table Header */}
            <div className="bg-slate-50 px-6 py-3 border-b border-slate-200">
              <div className="grid grid-cols-12 gap-4 text-sm font-medium text-slate-600">
                <div className="col-span-5">Name & Chunks</div>
                <div className="col-span-2">Status</div>
                <div className="col-span-2">Size</div>
                <div className="col-span-3">Progress & Actions</div>
              </div>
            </div>
            
            {/* Chunked File Rows */}
            <div className="divide-y divide-slate-100">
              {files.map(file => (
                <ChunkedFileRow
                  key={file.id}
                  file={file}
                  onRemove={removeFile}
                  onRetry={retryFile}
                />
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}