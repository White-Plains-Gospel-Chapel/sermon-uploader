// Frontend version information
export const FRONTEND_VERSION = {
  version: "1.1.0",
  service: "sermon-uploader-frontend",
  fullVersion: "1.1.0-frontend",
  features: {
    largeFileUpload: true,
    cloudflareBypass: true,
    parallelUploads: true,
    duplicateDetection: true,
    realtimeProgress: true,
    versionTracking: true,
  }
}

// Check if frontend and backend versions match
export function checkVersionCompatibility(backendVersion: string): boolean {
  return backendVersion === FRONTEND_VERSION.version
}