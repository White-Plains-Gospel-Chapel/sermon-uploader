"use client"

import { ErrorBoundary } from '@/components/common/ErrorBoundary'
import { UploadManagerOptimized } from '@/components/upload/UploadManagerOptimized'
import { VersionDisplay } from '@/components/common/VersionDisplay'
import { UI_TEXT } from '@/utils/constants'

export default function UploadPage() {
  return (
    <ErrorBoundary>
      <div className="min-h-screen bg-slate-50">
        <div className="max-w-6xl mx-auto p-6">
          <header className="mb-8">
            <h1 className="text-3xl font-bold text-slate-900 mb-2">
              {UI_TEXT.UPLOAD.TITLE}
            </h1>
            <p className="text-slate-600">
              {UI_TEXT.UPLOAD.SUBTITLE} - Parallel Processing Optimized
            </p>
            <div className="mt-2 flex gap-4 text-sm text-slate-500">
              <span>✓ 2-5 parallel uploads</span>
              <span>✓ Raspberry Pi optimized</span>
              <span>✓ Duplicate detection</span>
              <span>✓ Real-time progress</span>
            </div>
            <div className="mt-3 pt-3 border-t border-slate-200">
              <VersionDisplay />
            </div>
          </header>
          
          <main>
            <UploadManagerOptimized />
          </main>
        </div>
      </div>
    </ErrorBoundary>
  )
}