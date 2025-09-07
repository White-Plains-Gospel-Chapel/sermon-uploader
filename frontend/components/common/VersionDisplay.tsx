"use client"

import { useState, useEffect } from 'react'
import { FRONTEND_VERSION, checkVersionCompatibility } from '@/utils/version'

interface VersionInfo {
  version: string
  service: string
  fullVersion: string
  buildTime: string
  gitCommit: string
  environment: string
  features: Record<string, any>
}

export function VersionDisplay() {
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchVersionInfo()
  }, [])

  const fetchVersionInfo = async () => {
    try {
      const response = await fetch('/api/version')
      if (!response.ok) {
        throw new Error(`Failed to fetch version: ${response.statusText}`)
      }
      const data = await response.json()
      setVersionInfo(data)
      
      // Check version compatibility
      if (!checkVersionCompatibility(data.version)) {
        console.warn(`Version mismatch: Frontend ${FRONTEND_VERSION.version}, Backend ${data.version}`)
      }
    } catch (err) {
      console.error('Error fetching version:', err)
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="text-xs text-slate-400">
        Loading version...
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-xs text-red-500">
        Version unavailable
      </div>
    )
  }

  if (!versionInfo) {
    return null
  }

  const isProduction = versionInfo.environment === 'production'
  const envColor = isProduction ? 'text-green-600' : 'text-orange-600'
  const versionMatch = checkVersionCompatibility(versionInfo.version)

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2 text-xs">
        <span className="text-slate-500">Frontend:</span>
        <span className="font-mono text-slate-700">{FRONTEND_VERSION.fullVersion}</span>
      </div>
      <div className="flex items-center gap-2 text-xs">
        <span className="text-slate-500">Backend:</span>
        <span className="font-mono text-slate-700">{versionInfo.fullVersion}</span>
        <span className={`ml-2 ${envColor}`}>
          ({versionInfo.environment})
        </span>
        {versionInfo.gitCommit !== 'unknown' && (
          <span className="text-slate-400 font-mono">
            @{versionInfo.gitCommit.substring(0, 7)}
          </span>
        )}
      </div>
      {!versionMatch && (
        <div className="text-xs text-amber-600 flex items-center gap-1">
          <span>⚠️</span>
          <span>Version mismatch detected</span>
        </div>
      )}
    </div>
  )
}