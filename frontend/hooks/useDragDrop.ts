import { useState, useCallback, DragEvent } from 'react'

interface UseDragDropOptions {
  onDrop: (files: File[]) => void
  accept?: string[]
}

export function useDragDrop({ onDrop, accept }: UseDragDropOptions) {
  const [isDragging, setIsDragging] = useState(false)
  const [dragCounter, setDragCounter] = useState(0)

  const handleDragEnter = useCallback((e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    
    setDragCounter(prev => prev + 1)
    
    if (e.dataTransfer.items && e.dataTransfer.items.length > 0) {
      setIsDragging(true)
    }
  }, [])

  const handleDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    
    setDragCounter(prev => {
      const newCounter = prev - 1
      if (newCounter === 0) {
        setIsDragging(false)
      }
      return newCounter
    })
  }, [])

  const handleDragOver = useCallback((e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
  }, [])

  const handleDrop = useCallback((e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    
    setIsDragging(false)
    setDragCounter(0)
    
    const files = Array.from(e.dataTransfer.files)
    
    if (accept && accept.length > 0) {
      const filteredFiles = files.filter(file => {
        const extension = `.${file.name.split('.').pop()?.toLowerCase()}`
        return accept.includes(extension)
      })
      onDrop(filteredFiles)
    } else {
      onDrop(files)
    }
  }, [onDrop, accept])

  return {
    isDragging,
    dragHandlers: {
      onDragEnter: handleDragEnter,
      onDragLeave: handleDragLeave,
      onDragOver: handleDragOver,
      onDrop: handleDrop
    }
  }
}