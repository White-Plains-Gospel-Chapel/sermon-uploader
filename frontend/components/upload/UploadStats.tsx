import React from 'react'
import { CheckCircle, AlertCircle, Copy, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface UploadStatsProps {
  stats: {
    total: number
    completed: number
    failed: number
    duplicates: number
  }
  onClearCompleted?: () => void
}

export function UploadStats({ stats, onClearCompleted }: UploadStatsProps) {
  const hasCompleted = stats.completed > 0 || stats.failed > 0 || stats.duplicates > 0
  
  if (stats.total === 0) return null

  return (
    <div className="flex items-center justify-between p-4 bg-slate-50 rounded-lg">
      <div className="flex gap-6 text-sm">
        {stats.completed > 0 && (
          <div className="flex items-center gap-2">
            <CheckCircle className="h-4 w-4 text-green-600" />
            <span className="text-slate-700">
              {stats.completed} completed
            </span>
          </div>
        )}
        
        {stats.failed > 0 && (
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 text-red-600" />
            <span className="text-slate-700">
              {stats.failed} failed
            </span>
          </div>
        )}
        
        {stats.duplicates > 0 && (
          <div className="flex items-center gap-2">
            <Copy className="h-4 w-4 text-yellow-600" />
            <span className="text-slate-700">
              {stats.duplicates} duplicates
            </span>
          </div>
        )}
      </div>
      
      {hasCompleted && onClearCompleted && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClearCompleted}
          className="gap-2"
        >
          <Trash2 className="h-4 w-4" />
          Clear completed
        </Button>
      )}
    </div>
  )
}