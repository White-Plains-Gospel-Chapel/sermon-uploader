'use client'

import { useState, useCallback } from 'react'
import { useDropzone } from 'react-dropzone'
import UploadProgress from '@/components/upload/UploadProgress'
import FileList from '@/components/upload/FileList'

export default function SermonUploadPage() {
  const [files, setFiles] = useState<File[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<{[key: string]: number}>({})

  const onDrop = useCallback((acceptedFiles: File[]) => {
    setFiles(prev => [...prev, ...acceptedFiles])
  }, [])

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept: {
      'audio/*': ['.wav', '.mp3', '.aac', '.m4a'],
    },
    multiple: true,
  })

  const handleUpload = async () => {
    setUploading(true)
    console.log('Starting upload for', files.length, 'files')
    
    for (const file of files) {
      const formData = new FormData()
      formData.append('file', file)
      
      try {
        const xhr = new XMLHttpRequest()
        
        xhr.upload.addEventListener('progress', (e) => {
          if (e.lengthComputable) {
            const percentComplete = (e.loaded / e.total) * 100
            console.log(`Upload progress for ${file.name}: ${percentComplete.toFixed(2)}%`)
            setUploadProgress(prev => ({
              ...prev,
              [file.name]: percentComplete
            }))
          }
        })
        
        xhr.onload = () => {
          console.log(`Upload response for ${file.name}: Status ${xhr.status}`, xhr.responseText)
          if (xhr.status === 200) {
            console.log(`✅ Upload successful: ${file.name}`)
            alert(`✅ Upload successful: ${file.name}`)
          } else {
            console.error(`❌ Upload failed for ${file.name}: Status ${xhr.status}`)
            alert(`❌ Upload failed for ${file.name}: ${xhr.responseText}`)
          }
        }
        
        xhr.onerror = () => {
          console.error(`❌ Network error uploading ${file.name}`)
          alert(`❌ Network error uploading ${file.name}`)
        }
        
        // Use the correct API URL - either from env or direct to backend
        const apiUrl = typeof window !== 'undefined' && window.location.hostname === 'admin.wpgc.church' 
          ? 'http://api.wpgc.church' 
          : (process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000')
        
        const uploadUrl = `${apiUrl}/api/uploads/sermon`
        console.log(`Uploading ${file.name} to: ${uploadUrl}`)
        
        xhr.open('POST', uploadUrl)
        xhr.send(formData)
        
      } catch (error) {
        console.error(`Error uploading ${file.name}:`, error)
      }
    }
    
    setUploading(false)
  }

  const removeFile = (fileName: string) => {
    setFiles(prev => prev.filter(f => f.name !== fileName))
    setUploadProgress(prev => {
      const newProgress = { ...prev }
      delete newProgress[fileName]
      return newProgress
    })
  }

  return (
    <div className="max-w-6xl mx-auto">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Upload Sermons</h1>
        <p className="text-gray-600 mt-2">Upload sermon audio files to the church library</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <div
            {...getRootProps()}
            className={`border-2 border-dashed rounded-lg p-12 text-center cursor-pointer transition-colors ${
              isDragActive
                ? 'border-purple-500 bg-purple-50'
                : 'border-gray-300 hover:border-purple-400 bg-white'
            }`}
          >
            <input {...getInputProps()} />
            <div className="mb-4">
              <span className="text-6xl">🎵</span>
            </div>
            <p className="text-xl font-medium text-gray-700 mb-2">
              {isDragActive
                ? 'Drop the files here...'
                : 'Drag & drop sermon files here'}
            </p>
            <p className="text-sm text-gray-500">
              or click to select files
            </p>
            <p className="text-xs text-gray-400 mt-4">
              Supported formats: WAV, MP3, AAC, M4A
            </p>
          </div>

          {files.length > 0 && (
            <div className="mt-6 bg-white rounded-lg shadow-sm p-6">
              <h2 className="text-lg font-semibold mb-4">Files to Upload ({files.length})</h2>
              <FileList 
                files={files} 
                uploadProgress={uploadProgress}
                onRemove={removeFile}
              />
              
              <div className="mt-6 flex justify-end">
                <button
                  onClick={() => setFiles([])}
                  className="mr-4 px-4 py-2 text-gray-600 hover:text-gray-800"
                  disabled={uploading}
                >
                  Clear All
                </button>
                <button
                  onClick={handleUpload}
                  disabled={uploading || files.length === 0}
                  className="px-6 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {uploading ? 'Uploading...' : `Upload ${files.length} Files`}
                </button>
              </div>
            </div>
          )}
        </div>

        <div className="lg:col-span-1">
          <div className="bg-white rounded-lg shadow-sm p-6 mb-6">
            <h3 className="text-lg font-semibold mb-4">Upload Statistics</h3>
            <div className="space-y-3">
              <div>
                <p className="text-sm text-gray-600">Total Files</p>
                <p className="text-2xl font-bold">{files.length}</p>
              </div>
              <div>
                <p className="text-sm text-gray-600">Total Size</p>
                <p className="text-2xl font-bold">
                  {formatBytes(files.reduce((acc, file) => acc + file.size, 0))}
                </p>
              </div>
              {uploading && (
                <div>
                  <p className="text-sm text-gray-600">Upload Speed</p>
                  <p className="text-2xl font-bold">-- MB/s</p>
                </div>
              )}
            </div>
          </div>

          <div className="bg-blue-50 rounded-lg p-6">
            <h3 className="text-lg font-semibold mb-2 text-blue-900">Quick Tips</h3>
            <ul className="text-sm text-blue-800 space-y-2">
              <li>• Upload multiple files at once</li>
              <li>• Files are automatically processed</li>
              <li>• Duplicates are detected automatically</li>
              <li>• Maximum file size: 2GB per file</li>
            </ul>
          </div>
        </div>
      </div>
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