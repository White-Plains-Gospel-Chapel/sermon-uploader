import '@testing-library/jest-dom'
import { beforeAll, afterEach, afterAll } from 'vitest'
import { cleanup } from '@testing-library/react'

// Global test setup
beforeAll(() => {
  // Mock window.matchMedia for tests
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => {},
    }),
  })

  // Mock ResizeObserver
  global.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }

  // Mock File API
  global.File = class File {
    name: string
    size: number
    type: string
    lastModified: number

    constructor(chunks: any[], filename: string, options: any = {}) {
      this.name = filename
      this.size = chunks.reduce((acc, chunk) => acc + chunk.length, 0)
      this.type = options.type || ''
      this.lastModified = options.lastModified || Date.now()
    }
  } as any

  // Mock DataTransfer for drag and drop tests
  global.DataTransfer = class DataTransfer {
    dropEffect: string = 'none'
    effectAllowed: string = 'all'
    files: FileList = [] as any
    items: any[] = []
    types: string[] = []

    constructor() {}

    clearData() {}
    getData() { return '' }
    setData() {}
    setDragImage() {}
  } as any

  // Mock FileReader
  global.FileReader = class FileReader {
    readyState: number = 0
    result: string | ArrayBuffer | null = null
    error: any = null
    onload: any = null
    onerror: any = null
    onabort: any = null

    readAsDataURL() {
      this.result = 'data:text/plain;base64,dGVzdA=='
      if (this.onload) this.onload({ target: this })
    }

    readAsText() {
      this.result = 'test content'
      if (this.onload) this.onload({ target: this })
    }

    abort() {}
  } as any
})

// Clean up after each test
afterEach(() => {
  cleanup()
})