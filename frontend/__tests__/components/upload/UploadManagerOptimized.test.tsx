import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UploadManagerOptimized } from '@/components/upload/UploadManagerOptimized'

// Define the file type for the mock
type MockFile = {
  id: string
  file: File
  status: string
  progress: number
}

// Mock the custom hook
const mockUseUploadQueueOptimized = {
  files: [] as MockFile[],
  isProcessing: false,
  addFiles: vi.fn(),
  removeFile: vi.fn(),
  clearCompleted: vi.fn(),
  stats: {
    total: 0,
    completed: 0,
    failed: 0,
    duplicates: 0,
    uploading: 0
  },
  performance: {
    speed: '0 KB/s',
    timeRemaining: 'Calculating...',
    progress: 0,
    concurrency: 0
  }
}

vi.mock('@/hooks/useUploadQueueOptimized', () => ({
  useUploadQueueOptimized: () => mockUseUploadQueueOptimized
}))

// Mock the child components
vi.mock('@/components/upload/UploadDropzone', () => ({
  UploadDropzone: ({ onFilesSelected, disabled }: any) => (
    <div data-testid="upload-dropzone">
      <input
        data-testid="file-input"
        type="file"
        multiple
        onChange={(e) => {
          const files = Array.from(e.target.files || [])
          onFilesSelected(files)
        }}
        disabled={disabled}
      />
    </div>
  )
}))

vi.mock('@/components/upload/UploadQueue', () => ({
  UploadQueue: ({ files, onRemoveFile }: any) => (
    <div data-testid="upload-queue">
      {files.map((file: any) => (
        <div key={file.id} data-testid={`file-${file.id}`}>
          <span>{file.file.name}</span>
          <button onClick={() => onRemoveFile(file.id)}>Remove</button>
        </div>
      ))}
    </div>
  )
}))

vi.mock('@/components/upload/UploadStats', () => ({
  UploadStats: ({ stats, onClearCompleted }: any) => (
    <div data-testid="upload-stats">
      <span data-testid="stats-total">{stats.total}</span>
      <span data-testid="stats-completed">{stats.completed}</span>
      <button onClick={onClearCompleted} data-testid="clear-completed">
        Clear Completed
      </button>
    </div>
  )
}))

describe('UploadManagerOptimized', () => {
  const user = userEvent.setup()
  
  beforeEach(() => {
    vi.clearAllMocks()
    // Reset the mock to default state
    Object.assign(mockUseUploadQueueOptimized, {
      files: [],
      isProcessing: false,
      stats: {
        total: 0,
        completed: 0,
        failed: 0,
        duplicates: 0,
        uploading: 0
      },
      performance: {
        speed: '0 KB/s',
        timeRemaining: 'Calculating...',
        progress: 0,
        concurrency: 0
      }
    })
  })

  it('renders the upload dropzone', () => {
    render(<UploadManagerOptimized />)
    
    expect(screen.getByTestId('upload-dropzone')).toBeInTheDocument()
    expect(screen.getByTestId('upload-queue')).toBeInTheDocument()
  })

  it('passes onFilesSelected to UploadDropzone', async () => {
    render(<UploadManagerOptimized />)
    
    const fileInput = screen.getByTestId('file-input')
    const testFile = new File(['test content'], 'test.wav', { type: 'audio/wav' })
    
    await user.upload(fileInput, testFile)
    
    expect(mockUseUploadQueueOptimized.addFiles).toHaveBeenCalledWith([testFile])
  })

  it('disables dropzone when processing', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    
    render(<UploadManagerOptimized />)
    
    const fileInput = screen.getByTestId('file-input')
    expect(fileInput).toBeDisabled()
  })

  it('shows performance metrics when processing', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    mockUseUploadQueueOptimized.stats = {
      ...mockUseUploadQueueOptimized.stats,
      uploading: 3,
      completed: 2,
      total: 5
    }
    mockUseUploadQueueOptimized.performance = {
      speed: '1.5 MB/s',
      timeRemaining: '2m 30s',
      progress: 75,
      concurrency: 3
    }
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByText('Processing 3 files in parallel')).toBeInTheDocument()
    expect(screen.getByText('2 of 5 completed')).toBeInTheDocument()
    expect(screen.getByText('3 concurrent')).toBeInTheDocument()
    expect(screen.getByText('75% complete')).toBeInTheDocument()
    expect(screen.getByText('2m 30s remaining')).toBeInTheDocument()
    expect(screen.getByText('1.5 MB/s')).toBeInTheDocument()
    expect(screen.getByText('3 active')).toBeInTheDocument()
  })

  it('hides performance metrics when not processing', () => {
    mockUseUploadQueueOptimized.isProcessing = false
    
    render(<UploadManagerOptimized />)
    
    expect(screen.queryByText(/Processing.*files in parallel/)).not.toBeInTheDocument()
    expect(screen.queryByText(/concurrent$/)).not.toBeInTheDocument()
  })

  it('shows upload stats when files are present', () => {
    mockUseUploadQueueOptimized.stats = {
      total: 5,
      completed: 3,
      failed: 1,
      duplicates: 0,
      uploading: 1
    }
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByTestId('upload-stats')).toBeInTheDocument()
    expect(screen.getByTestId('stats-total')).toHaveTextContent('5')
    expect(screen.getByTestId('stats-completed')).toHaveTextContent('3')
  })

  it('hides upload stats when no files are present', () => {
    mockUseUploadQueueOptimized.stats = {
      total: 0,
      completed: 0,
      failed: 0,
      duplicates: 0,
      uploading: 0
    }
    
    render(<UploadManagerOptimized />)
    
    expect(screen.queryByTestId('upload-stats')).not.toBeInTheDocument()
  })

  it('calls clearCompleted when clear completed button is clicked', async () => {
    mockUseUploadQueueOptimized.stats = {
      total: 3,
      completed: 2,
      failed: 1,
      duplicates: 0,
      uploading: 0
    }
    
    render(<UploadManagerOptimized />)
    
    const clearButton = screen.getByTestId('clear-completed')
    await user.click(clearButton)
    
    expect(mockUseUploadQueueOptimized.clearCompleted).toHaveBeenCalled()
  })

  it('passes files and removeFile to UploadQueue', () => {
    const mockFiles = [
      { id: '1', file: new File(['content'], 'test1.wav'), status: 'queued', progress: 0 },
      { id: '2', file: new File(['content'], 'test2.wav'), status: 'uploading', progress: 50 }
    ]
    mockUseUploadQueueOptimized.files = mockFiles
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByTestId('file-1')).toBeInTheDocument()
    expect(screen.getByTestId('file-2')).toBeInTheDocument()
    expect(screen.getByText('test1.wav')).toBeInTheDocument()
    expect(screen.getByText('test2.wav')).toBeInTheDocument()
  })

  it('calls removeFile when remove button is clicked', async () => {
    const mockFiles = [
      { id: '1', file: new File(['content'], 'test.wav'), status: 'queued', progress: 0 }
    ]
    mockUseUploadQueueOptimized.files = mockFiles
    
    render(<UploadManagerOptimized />)
    
    const removeButton = screen.getByText('Remove')
    await user.click(removeButton)
    
    expect(mockUseUploadQueueOptimized.removeFile).toHaveBeenCalledWith('1')
  })

  it('displays correct concurrency badge text', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    mockUseUploadQueueOptimized.performance.concurrency = 4
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByText('4 concurrent')).toBeInTheDocument()
  })

  it('displays loading spinner when processing', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    
    render(<UploadManagerOptimized />)
    
    // Check for the loader icon's animation class
    const loader = document.querySelector('.lucide-loader2')
    expect(loader).toHaveClass('animate-spin')
  })

  it('renders performance metrics grid with correct structure', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    mockUseUploadQueueOptimized.performance = {
      speed: '2.1 MB/s',
      timeRemaining: '1m 45s',
      progress: 60,
      concurrency: 2
    }
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByText('Upload Speed')).toBeInTheDocument()
    expect(screen.getByText('Parallel Uploads')).toBeInTheDocument()
    expect(screen.getByText('Time Remaining')).toBeInTheDocument()
    expect(screen.getByText('2.1 MB/s')).toBeInTheDocument()
    expect(screen.getByText('2 active')).toBeInTheDocument()
    expect(screen.getByText('1m 45s')).toBeInTheDocument()
  })

  it('handles zero progress correctly', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    mockUseUploadQueueOptimized.performance.progress = 0
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByText('0% complete')).toBeInTheDocument()
  })

  it('handles 100% progress correctly', () => {
    mockUseUploadQueueOptimized.isProcessing = true
    mockUseUploadQueueOptimized.performance.progress = 100
    
    render(<UploadManagerOptimized />)
    
    expect(screen.getByText('100% complete')).toBeInTheDocument()
  })
})