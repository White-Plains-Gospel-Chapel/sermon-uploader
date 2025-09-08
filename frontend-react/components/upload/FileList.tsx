interface FileListProps {
  files: File[]
  uploadProgress: { [key: string]: number }
  onRemove: (fileName: string) => void
}

export default function FileList({ files, uploadProgress, onRemove }: FileListProps) {
  return (
    <div className="space-y-3">
      {files.map((file) => (
        <div
          key={file.name}
          className="flex items-center justify-between p-4 bg-gray-50 rounded-lg"
        >
          <div className="flex items-center flex-1">
            <span className="text-2xl mr-3">🎵</span>
            <div className="flex-1">
              <p className="font-medium text-gray-900">{file.name}</p>
              <p className="text-sm text-gray-500">
                {formatBytes(file.size)}
              </p>
              {uploadProgress[file.name] !== undefined && (
                <div className="mt-2">
                  <div className="bg-gray-200 rounded-full h-2 overflow-hidden">
                    <div
                      className="bg-purple-600 h-full transition-all duration-300"
                      style={{ width: `${uploadProgress[file.name]}%` }}
                    />
                  </div>
                  <p className="text-xs text-gray-600 mt-1">
                    {Math.round(uploadProgress[file.name])}%
                  </p>
                </div>
              )}
            </div>
          </div>
          <button
            onClick={() => onRemove(file.name)}
            className="ml-4 text-red-500 hover:text-red-700"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}

function formatBytes(bytes: number, decimals = 2) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}