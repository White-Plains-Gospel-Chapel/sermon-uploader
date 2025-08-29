/**
 * Limits the number of concurrent promises
 * Essential for Raspberry Pi resource management
 */
export class ConcurrencyLimiter {
  private running = 0
  private queue: Array<() => void> = []
  
  constructor(private maxConcurrent: number) {}
  
  async run<T>(fn: () => Promise<T>): Promise<T> {
    while (this.running >= this.maxConcurrent) {
      await new Promise<void>(resolve => this.queue.push(resolve))
    }
    
    this.running++
    
    try {
      return await fn()
    } finally {
      this.running--
      const next = this.queue.shift()
      if (next) next()
    }
  }
  
  get pending(): number {
    return this.queue.length
  }
  
  get active(): number {
    return this.running
  }
}

/**
 * Detects optimal concurrency for current device
 * Lower for Raspberry Pi, higher for desktop
 */
export function getOptimalConcurrency(): number {
  // Check if we're on a mobile/embedded device
  const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
    navigator.userAgent
  )
  
  // Check available cores
  const cores = navigator.hardwareConcurrency || 2
  
  // Check available memory (if API is available)
  const memory = (performance as any).memory?.jsHeapSizeLimit
  const isLowMemory = memory && memory < 500 * 1024 * 1024 // Less than 500MB
  
  // Determine optimal concurrency
  if (isMobile || isLowMemory) {
    return 2 // Conservative for Pi/mobile
  } else if (cores <= 4) {
    return 3 // Mid-range devices
  } else {
    return 5 // High-end devices
  }
}

/**
 * Delays execution by specified milliseconds
 */
export function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

/**
 * Implements exponential backoff for retries
 */
export function getRetryDelay(attempt: number): number {
  const baseDelay = 1000 // 1 second
  const maxDelay = 30000 // 30 seconds
  return Math.min(baseDelay * Math.pow(2, attempt), maxDelay)
}