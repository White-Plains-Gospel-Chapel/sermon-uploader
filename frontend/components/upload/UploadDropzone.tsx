import React, { useRef } from 'react'
import { Upload, FileAudio } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { useDragDrop } from '@/hooks/useDragDrop'
import { UI_TEXT, UPLOAD_CONFIG } from '@/utils/constants'

interface UploadDropzoneProps {
  onFilesSelected: (files: File[]) => void
  disabled?: boolean
}

export function UploadDropzone({ onFilesSelected, disabled = false }: UploadDropzoneProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  
  const { isDragging, dragHandlers } = useDragDrop({
    onDrop: onFilesSelected,
    accept: UPLOAD_CONFIG.ALLOWED_EXTENSIONS
  })

  const handleClick = () => {
    if (!disabled) {
      fileInputRef.current?.click()
    }
  }

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    onFilesSelected(files)
    // Reset input value to allow selecting the same file again
    e.target.value = ''
  }

  return (
    <Card className="overflow-hidden">
      <CardContent className="p-0">
        <div
          {...dragHandlers}
          onClick={handleClick}
          className={`
            relative min-h-[300px] p-12 text-center cursor-pointer
            border-2 border-dashed transition-all duration-200
            ${isDragging 
              ? 'border-primary bg-primary/5' 
              : 'border-slate-300 hover:border-slate-400 bg-white hover:bg-slate-50'
            }
            ${disabled ? 'opacity-50 cursor-not-allowed' : ''}
          `}
        >
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept={UPLOAD_CONFIG.ALLOWED_EXTENSIONS.join(',')}
            onChange={handleFileSelect}
            className="hidden"
            disabled={disabled}
          />
          
          <div className="flex flex-col items-center justify-center space-y-4">
            <div className={`p-4 rounded-full ${isDragging ? 'bg-primary/10' : 'bg-slate-100'}`}>
              <Upload className={`h-12 w-12 ${isDragging ? 'text-primary' : 'text-slate-500'}`} />
            </div>
            
            <div className="space-y-2">
              <h3 className="text-xl font-semibold text-slate-800">
                {isDragging ? UI_TEXT.UPLOAD.DRAG_ACTIVE : UI_TEXT.UPLOAD.DRAG_INACTIVE}
              </h3>
              <p className="text-slate-600">
                or <span className="text-primary font-medium hover:underline">
                  {UI_TEXT.UPLOAD.BROWSE}
                </span> your computer
              </p>
            </div>
            
            <div className="flex gap-6 mt-4 text-sm text-slate-500">
              <div className="flex items-center gap-2">
                <FileAudio className="h-4 w-4" />
                <span>WAV files only</span>
              </div>
              <div className="flex items-center gap-2">
                <Upload className="h-4 w-4" />
                <span>Up to 2GB per file</span>
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}