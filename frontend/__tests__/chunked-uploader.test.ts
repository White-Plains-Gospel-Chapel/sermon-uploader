import { ChunkedUploader, UploadProgress, ChunkStrategy } from '../lib/chunked-uploader';
import { api } from '../lib/api';

// Mock the API module
jest.mock('../lib/api');

describe('ChunkedUploader', () => {
  let uploader: ChunkedUploader;
  let mockApi: jest.Mocked<typeof api>;
  let onProgress: jest.Mock;
  let onError: jest.Mock;

  beforeEach(() => {
    mockApi = api as jest.Mocked<typeof api>;
    onProgress = jest.fn();
    onError = jest.fn();
    
    uploader = new ChunkedUploader({
      onProgress,
      onError,
      chunkStrategy: 'fixed',
      chunkSize: 10 * 1024 * 1024, // 10MB
      maxConcurrent: 2,
      maxRetries: 3,
    });

    // Clear all mocks
    jest.clearAllMocks();
  });

  describe('File Chunking', () => {
    test('should split 700MB file into 70 chunks of 10MB each', () => {
      // Create a mock 700MB file
      const fileSize = 734003200; // 700MB
      const file = new File([''], 'sermon.wav', { type: 'audio/wav' });
      Object.defineProperty(file, 'size', { value: fileSize });

      const chunks = uploader.splitIntoChunks(file);

      expect(chunks).toHaveLength(70);
      expect(chunks[0].size).toBe(10 * 1024 * 1024);
      expect(chunks[69].size).toBe(4003200); // Remaining bytes
    });

    test('should handle small files (< chunk size)', () => {
      const file = new File(['x'.repeat(5 * 1024 * 1024)], 'small.wav');
      
      const chunks = uploader.splitIntoChunks(file);

      expect(chunks).toHaveLength(1);
      expect(chunks[0].size).toBe(5 * 1024 * 1024);
    });

    test('should use adaptive chunk sizing based on network speed', async () => {
      const adaptiveUploader = new ChunkedUploader({
        onProgress,
        onError,
        chunkStrategy: 'adaptive',
        maxConcurrent: 2,
      });

      // Mock network speed test
      const file = new File([''], 'test.wav', { type: 'audio/wav' });
      Object.defineProperty(file, 'size', { value: 100 * 1024 * 1024 }); // 100MB

      // Simulate fast network (>10MB/s)
      jest.spyOn(adaptiveUploader, 'measureNetworkSpeed').mockResolvedValue(15); // 15MB/s

      const chunkSize = await adaptiveUploader.determineOptimalChunkSize(file);
      
      expect(chunkSize).toBe(25 * 1024 * 1024); // Should use 25MB chunks for fast network
    });
  });

  describe('Upload Initialization', () => {
    test('should check for duplicate files before uploading', async () => {
      const file = new File(['test'], 'sermon.wav');
      const fileHash = 'abc123';

      jest.spyOn(uploader, 'calculateHash').mockResolvedValue(fileHash);
      mockApi.checkDuplicate.mockResolvedValue({ isDuplicate: true });

      await expect(uploader.uploadFile(file)).rejects.toThrow('File already exists');
      
      expect(mockApi.checkDuplicate).toHaveBeenCalledWith('sermon.wav', fileHash);
      expect(mockApi.initiateMultipartUpload).not.toHaveBeenCalled();
    });

    test('should initialize multipart upload for new files', async () => {
      const file = new File(['test'], 'sermon.wav');
      const fileHash = 'abc123';

      jest.spyOn(uploader, 'calculateHash').mockResolvedValue(fileHash);
      mockApi.checkDuplicate.mockResolvedValue({ isDuplicate: false });
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 1,
      });
      mockApi.getPresignedURL.mockResolvedValue({
        url: 'https://minio.example.com/presigned',
        partNumber: 1,
      });

      // Mock successful chunk upload
      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'ETag': 'etag-1' }),
      });

      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      await uploader.uploadFile(file);

      expect(mockApi.initiateMultipartUpload).toHaveBeenCalledWith({
        filename: 'sermon.wav',
        fileSize: file.size,
        chunkSize: 10 * 1024 * 1024,
        fileHash,
      });
    });
  });

  describe('Concurrent Upload Management', () => {
    test('should limit concurrent uploads to maxConcurrent setting', async () => {
      const files = Array(5).fill(null).map((_, i) => 
        new File(['x'.repeat(10 * 1024 * 1024)], `sermon${i}.wav`)
      );

      // Track concurrent uploads
      let currentConcurrent = 0;
      let maxConcurrentObserved = 0;

      mockApi.initiateMultipartUpload.mockImplementation(async () => {
        currentConcurrent++;
        maxConcurrentObserved = Math.max(maxConcurrentObserved, currentConcurrent);
        
        // Simulate upload time
        await new Promise(resolve => setTimeout(resolve, 100));
        
        currentConcurrent--;
        return { uploadId: 'test', totalParts: 1 };
      });

      mockApi.getPresignedURL.mockResolvedValue({ url: 'test-url', partNumber: 1 });
      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'ETag': 'test' }),
      });
      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      // Upload all files
      await Promise.all(files.map(file => uploader.uploadFile(file)));

      expect(maxConcurrentObserved).toBeLessThanOrEqual(2); // maxConcurrent = 2
    });

    test('should queue uploads when max concurrent limit reached', async () => {
      const uploadQueue = uploader.getQueueStatus();
      
      // Start 5 uploads with max concurrent = 2
      const uploads = Array(5).fill(null).map((_, i) => {
        const file = new File(['test'], `sermon${i}.wav`);
        return uploader.uploadFile(file);
      });

      // Check queue status immediately
      const status = uploader.getQueueStatus();
      expect(status.active).toBeLessThanOrEqual(2);
      expect(status.queued).toBe(3);
    });
  });

  describe('Error Handling and Retry Logic', () => {
    test('should retry failed chunks with exponential backoff', async () => {
      const file = new File(['x'.repeat(10 * 1024 * 1024)], 'sermon.wav');
      
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 1,
      });

      mockApi.getPresignedURL.mockResolvedValue({
        url: 'https://minio.example.com/presigned',
        partNumber: 1,
      });

      // Mock fetch to fail twice, then succeed
      let attemptCount = 0;
      global.fetch = jest.fn().mockImplementation(() => {
        attemptCount++;
        if (attemptCount < 3) {
          return Promise.reject(new Error('Network error'));
        }
        return Promise.resolve({
          ok: true,
          headers: new Headers({ 'ETag': 'success-etag' }),
        });
      });

      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      const startTime = Date.now();
      await uploader.uploadFile(file);
      const duration = Date.now() - startTime;

      expect(attemptCount).toBe(3);
      // Should have delays: ~1s + ~2s = ~3s total (with jitter)
      expect(duration).toBeGreaterThan(2000);
      expect(duration).toBeLessThan(5000);
    });

    test('should fail after max retries exceeded', async () => {
      const file = new File(['test'], 'sermon.wav');
      
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 1,
      });

      mockApi.getPresignedURL.mockResolvedValue({
        url: 'https://minio.example.com/presigned',
        partNumber: 1,
      });

      // Always fail
      global.fetch = jest.fn().mockRejectedValue(new Error('Persistent network error'));

      await expect(uploader.uploadFile(file)).rejects.toThrow('Persistent network error');
      expect(global.fetch).toHaveBeenCalledTimes(3); // maxRetries = 3
    });

    test('should not retry on non-retryable errors (401, 403)', async () => {
      const file = new File(['test'], 'sermon.wav');
      
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 1,
      });

      mockApi.getPresignedURL.mockResolvedValue({
        url: 'https://minio.example.com/presigned',
        partNumber: 1,
      });

      // Return 403 Forbidden
      global.fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 403,
        statusText: 'Forbidden',
      });

      await expect(uploader.uploadFile(file)).rejects.toThrow('Forbidden');
      expect(global.fetch).toHaveBeenCalledTimes(1); // No retries for 403
    });
  });

  describe('Progress Tracking', () => {
    test('should report progress for each uploaded chunk', async () => {
      const file = new File(['x'.repeat(30 * 1024 * 1024)], 'sermon.wav'); // 30MB = 3 chunks
      
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 3,
      });

      mockApi.getPresignedURL.mockImplementation(async (_, partNumber) => ({
        url: `https://minio.example.com/presigned-${partNumber}`,
        partNumber,
      }));

      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'ETag': 'etag' }),
      });

      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      await uploader.uploadFile(file);

      // Should have progress updates for each chunk
      expect(onProgress).toHaveBeenCalledTimes(3);
      
      // Check progress values
      const progressCalls = onProgress.mock.calls.map(call => call[0]);
      expect(progressCalls[0].percentage).toBeCloseTo(33.33, 1);
      expect(progressCalls[1].percentage).toBeCloseTo(66.66, 1);
      expect(progressCalls[2].percentage).toBe(100);
    });

    test('should calculate upload speed and ETA', async () => {
      const file = new File(['x'.repeat(100 * 1024 * 1024)], 'sermon.wav'); // 100MB
      
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 10,
      });

      mockApi.getPresignedURL.mockImplementation(async (_, partNumber) => ({
        url: `https://minio.example.com/presigned-${partNumber}`,
        partNumber,
      }));

      // Simulate upload with delay
      global.fetch = jest.fn().mockImplementation(async () => {
        await new Promise(resolve => setTimeout(resolve, 100)); // 100ms per chunk
        return {
          ok: true,
          headers: new Headers({ 'ETag': 'etag' }),
        };
      });

      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      await uploader.uploadFile(file);

      // Check that speed and ETA were calculated
      const lastProgress = onProgress.mock.calls[onProgress.mock.calls.length - 1][0];
      expect(lastProgress.speed).toBeGreaterThan(0); // Speed in MB/s
      expect(lastProgress.eta).toBe(0); // ETA should be 0 when complete
    });
  });

  describe('Memory Management', () => {
    test('should not load entire file into memory', async () => {
      // Create a large file (700MB)
      const largeFile = new File([''], 'large-sermon.wav');
      Object.defineProperty(largeFile, 'size', { value: 734003200 });

      // Spy on file.slice to ensure we're chunking
      const sliceSpy = jest.spyOn(largeFile, 'slice');

      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 70,
      });

      // Mock other required methods
      mockApi.getPresignedURL.mockResolvedValue({ url: 'test', partNumber: 1 });
      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'ETag': 'test' }),
      });
      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      await uploader.uploadFile(largeFile);

      // Should have called slice for each chunk
      expect(sliceSpy).toHaveBeenCalledTimes(70);
      
      // Each slice should be 10MB or less
      sliceSpy.mock.calls.forEach((call, index) => {
        const [start, end] = call;
        const chunkSize = end - start;
        if (index < 69) {
          expect(chunkSize).toBe(10 * 1024 * 1024);
        } else {
          expect(chunkSize).toBeLessThanOrEqual(10 * 1024 * 1024);
        }
      });
    });

    test('should release blob references after upload', async () => {
      const file = new File(['x'.repeat(10 * 1024 * 1024)], 'sermon.wav');
      
      // Track blob URL creation and revocation
      const createObjectURLSpy = jest.spyOn(URL, 'createObjectURL');
      const revokeObjectURLSpy = jest.spyOn(URL, 'revokeObjectURL');

      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'upload-123',
        totalParts: 1,
      });

      mockApi.getPresignedURL.mockResolvedValue({ url: 'test', partNumber: 1 });
      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'ETag': 'test' }),
      });
      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      await uploader.uploadFile(file);

      // If blob URLs were created, they should be revoked
      const createCount = createObjectURLSpy.mock.calls.length;
      const revokeCount = revokeObjectURLSpy.mock.calls.length;
      expect(revokeCount).toBe(createCount);
    });
  });

  describe('Batch Upload Handling', () => {
    test('should handle batch of 10 × 700MB files without browser freeze', async () => {
      const files = Array(10).fill(null).map((_, i) => {
        const file = new File([''], `sermon${i}.wav`);
        Object.defineProperty(file, 'size', { value: 734003200 }); // 700MB each
        return file;
      });

      // Track memory and performance
      const startTime = Date.now();
      const uploadPromises: Promise<void>[] = [];

      // Mock API responses
      mockApi.initiateMultipartUpload.mockResolvedValue({
        uploadId: 'test',
        totalParts: 70,
      });
      mockApi.getPresignedURL.mockResolvedValue({ url: 'test', partNumber: 1 });
      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        headers: new Headers({ 'ETag': 'test' }),
      });
      mockApi.completeMultipartUpload.mockResolvedValue({ success: true });

      // Start batch upload
      for (const file of files) {
        uploadPromises.push(uploader.uploadFile(file));
      }

      // Should not throw or hang
      await expect(Promise.all(uploadPromises)).resolves.not.toThrow();

      // Verify reasonable performance (should process in parallel)
      const duration = Date.now() - startTime;
      expect(duration).toBeLessThan(60000); // Should complete in less than 60 seconds
    });
  });
});