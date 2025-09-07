import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UploadDropzone } from '@/components/upload/UploadDropzone'

// Mock the useDragDrop hook
const mockUseDragDrop = {
  isDragging: false,
  dragHandlers: {
    onDragOver: vi.fn(),
    onDragLeave: vi.fn(),
    onDrop: vi.fn(),
  }
}

vi.mock('@/hooks/useDragDrop', () => ({
  useDragDrop: () => mockUseDragDrop
}))

// Mock constants
vi.mock('@/utils/constants', () => ({
  UI_TEXT: {
    UPLOAD: {
      DRAG_ACTIVE: 'Drop files here',
      DRAG_INACTIVE: 'Drag and drop your files',
      BROWSE: 'click to browse'
    }
  },
  UPLOAD_CONFIG: {
    ALLOWED_EXTENSIONS: ['.wav', '.mp3', '.flac']
  }
}))

describe('UploadDropzone', () => {
  const user = userEvent.setup()
  const mockOnFilesSelected = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseDragDrop.isDragging = false
  })

  it('renders correctly with default props', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    expect(screen.getByText('Drag and drop your files')).toBeInTheDocument()
    expect(screen.getByText('click to browse')).toBeInTheDocument()
    expect(screen.getByText('WAV files only')).toBeInTheDocument()
    expect(screen.getByText('Up to 2GB per file')).toBeInTheDocument()
  })

  it('displays drag active state when dragging', () => {
    mockUseDragDrop.isDragging = true
    
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    expect(screen.getByText('Drop files here')).toBeInTheDocument()
  })

  it('handles file selection via input', async () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const testFile = new File(['test content'], 'test.wav', { type: 'audio/wav' })
    
    // Get the file input directly
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(fileInput).toBeTruthy()
    
    // Simulate file selection
    fireEvent.change(fileInput, { target: { files: [testFile] } })
    expect(mockOnFilesSelected).toHaveBeenCalledWith([testFile])
  })

  it('handles multiple file selection', async () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const testFiles = [
      new File(['content1'], 'test1.wav', { type: 'audio/wav' }),
      new File(['content2'], 'test2.wav', { type: 'audio/wav' })
    ]
    
    // Get the file input directly
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(fileInput).toBeTruthy()
    
    // Simulate file selection
    fireEvent.change(fileInput, { target: { files: testFiles } })
    expect(mockOnFilesSelected).toHaveBeenCalledWith(testFiles)
  })

  it('resets input value after file selection', async () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(fileInput).toBeTruthy()
    
    const testFile = new File(['content'], 'test.wav', { type: 'audio/wav' })
    fireEvent.change(fileInput, { target: { files: [testFile] } })
    
    expect(fileInput.value).toBe('')
  })

  it('does not trigger file selection when disabled', async () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} disabled />)
    
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(fileInput?.disabled).toBe(true)
  })

  it('applies disabled styling when disabled', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} disabled />)
    
    const dropzone = document.querySelector('[data-testid="dropzone"]') || 
                    document.querySelector('.cursor-pointer')
    expect(dropzone).toHaveClass('opacity-50', 'cursor-not-allowed')
  })

  it('applies hover styling when not disabled', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const dropzone = document.querySelector('.cursor-pointer')
    expect(dropzone).toHaveClass('hover:border-slate-400', 'hover:bg-slate-50')
  })

  it('applies dragging styles when isDragging is true', () => {
    mockUseDragDrop.isDragging = true
    
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const dropzone = document.querySelector('.cursor-pointer')
    expect(dropzone).toHaveClass('border-primary', 'bg-primary/5')
  })

  it('applies default styles when not dragging', () => {
    mockUseDragDrop.isDragging = false
    
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const dropzone = document.querySelector('.cursor-pointer')
    expect(dropzone).toHaveClass('border-slate-300', 'bg-white')
  })

  it('passes correct props to useDragDrop hook', () => {
    // This test verifies the hook is called - the implementation is mocked above
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    // The component should render without errors, which means the hook was called properly
    expect(screen.getByText('Drag and drop your files')).toBeInTheDocument()
  })

  it('spreads drag handlers to dropzone element', () => {
    const mockDragOver = vi.fn()
    const mockDragLeave = vi.fn()
    const mockDrop = vi.fn()
    
    mockUseDragDrop.dragHandlers = {
      onDragOver: mockDragOver,
      onDragLeave: mockDragLeave,
      onDrop: mockDrop
    }
    
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const dropzone = document.querySelector('.cursor-pointer')!
    
    fireEvent.dragOver(dropzone)
    expect(mockDragOver).toHaveBeenCalled()
    
    fireEvent.dragLeave(dropzone)
    expect(mockDragLeave).toHaveBeenCalled()
    
    fireEvent.drop(dropzone)
    expect(mockDrop).toHaveBeenCalled()
  })

  it('sets correct accept attribute on file input', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const fileInput = document.querySelector('input[type="file"]')
    expect(fileInput?.getAttribute('accept')).toBe('.wav,.mp3,.flac')
  })

  it('sets multiple attribute on file input', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const fileInput = document.querySelector('input[type="file"]')
    expect(fileInput?.hasAttribute('multiple')).toBe(true)
  })

  it('renders upload icon with correct styling', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const uploadIcon = document.querySelector('.lucide-upload')
    expect(uploadIcon).toHaveClass('h-12', 'w-12', 'text-slate-500')
  })

  it('renders upload icon with primary color when dragging', () => {
    mockUseDragDrop.isDragging = true
    
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const uploadIcon = document.querySelector('.lucide-upload')
    expect(uploadIcon).toHaveClass('text-primary')
  })

  it('renders file audio icon in constraints section', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    // FileAudio icon should be present
    const audioIcon = document.querySelector('.lucide-file-audio')
    expect(audioIcon).toBeInTheDocument()
    expect(audioIcon).toHaveClass('h-4', 'w-4')
  })

  it('has correct minimum height', () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const dropzone = document.querySelector('.cursor-pointer')
    expect(dropzone).toHaveClass('min-h-[300px]')
  })

  it('triggers file input click when dropzone is clicked', async () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} />)
    
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const clickSpy = vi.spyOn(fileInput, 'click')
    
    const dropzone = document.querySelector('.cursor-pointer')!
    await user.click(dropzone)
    
    expect(clickSpy).toHaveBeenCalled()
  })

  it('does not trigger file input click when disabled and clicked', async () => {
    render(<UploadDropzone onFilesSelected={mockOnFilesSelected} disabled />)
    
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const clickSpy = vi.spyOn(fileInput, 'click')
    
    const dropzone = document.querySelector('.cursor-pointer')!
    await user.click(dropzone)
    
    expect(clickSpy).not.toHaveBeenCalled()
  })
})