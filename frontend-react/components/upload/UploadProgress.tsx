interface UploadProgressProps {
  progress: number
  fileName: string
  status: 'uploading' | 'completed' | 'error'
}

export default function UploadProgress({ progress, fileName, status }: UploadProgressProps) {
  return (
    <div className="mb-4">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm font-medium text-gray-700">{fileName}</span>
        <span className="text-sm text-gray-500">{Math.round(progress)}%</span>
      </div>
      <div className="bg-gray-200 rounded-full h-2 overflow-hidden">
        <div
          className={`h-full transition-all duration-300 ${
            status === 'completed'
              ? 'bg-green-500'
              : status === 'error'
              ? 'bg-red-500'
              : 'bg-purple-600'
          }`}
          style={{ width: `${progress}%` }}
        />
      </div>
    </div>
  )
}