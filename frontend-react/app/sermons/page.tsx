'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'

interface Sermon {
  id: string
  filename: string
  size: number
  uploadDate: string
  duration?: number
  speaker?: string
}

export default function SermonsPage() {
  const [sermons, setSermons] = useState<Sermon[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchSermons()
  }, [])

  const fetchSermons = async () => {
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000'
      const response = await fetch(`${apiUrl}/api/admin/sermons`)
      const data = await response.json()
      setSermons(data.files || [])
    } catch (error) {
      console.error('Error fetching sermons:', error)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="mb-8 flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Sermon Library</h1>
          <p className="text-gray-600 mt-2">Manage and organize all uploaded sermons</p>
        </div>
        <Link
          href="/sermons/upload"
          className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
        >
          Upload New Sermon
        </Link>
      </div>

      {loading ? (
        <div className="flex justify-center items-center h-64">
          <div className="text-gray-500">Loading sermons...</div>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow-sm overflow-hidden">
          <table className="min-w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  File Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Size
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Upload Date
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Duration
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {sermons.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-4 text-center text-gray-500">
                    No sermons uploaded yet
                  </td>
                </tr>
              ) : (
                sermons.map((sermon) => (
                  <tr key={sermon.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <span className="text-2xl mr-3">🎵</span>
                        <div>
                          <div className="text-sm font-medium text-gray-900">
                            {sermon.filename}
                          </div>
                          {sermon.speaker && (
                            <div className="text-sm text-gray-500">
                              Speaker: {sermon.speaker}
                            </div>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {formatBytes(sermon.size)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {new Date(sermon.uploadDate).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {sermon.duration ? formatDuration(sermon.duration) : '--'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button className="text-purple-600 hover:text-purple-900 mr-3">
                        Download
                      </button>
                      <button className="text-red-600 hover:text-red-900">
                        Delete
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
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

function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  
  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  return `${minutes}m ${secs}s`
}