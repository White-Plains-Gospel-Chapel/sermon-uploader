//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

// TestEnvironment represents the test environment setup
type TestEnvironment struct {
	Config         *config.Config
	MinIOService   *services.MinIOService
	TestBucket     string
	TempDir        string
	Cleanup        []func()
	MinIOContainer testcontainers.Container // For containerized testing
}

// NewTestEnvironment creates a new test environment
func NewTestEnvironment(useContainer bool) (*TestEnvironment, error) {
	te := &TestEnvironment{
		Cleanup: make([]func(), 0),
	}

	if useContainer {
		if err := te.setupMinIOContainer(); err != nil {
			return nil, fmt.Errorf("failed to setup MinIO container: %w", err)
		}
	} else {
		te.setupLocalMinIO()
	}

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sermon-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	te.TempDir = tempDir
	te.Cleanup = append(te.Cleanup, func() { os.RemoveAll(tempDir) })

	// Initialize MinIO service
	te.MinIOService = services.NewMinIOService(te.Config)

	// Ensure test bucket exists
	if err := te.MinIOService.EnsureBucketExists(); err != nil {
		return nil, fmt.Errorf("failed to create test bucket: %w", err)
	}

	return te, nil
}

// setupLocalMinIO sets up configuration for local MinIO instance
func (te *TestEnvironment) setupLocalMinIO() {
	te.Config = config.New()
	te.TestBucket = te.Config.MinioBucket + "-test-" + fmt.Sprintf("%d", time.Now().Unix())
	te.Config.MinioBucket = te.TestBucket

	// Override config for testing
	te.Config.MaxConcurrentUploads = 10
	te.Config.ChunkSize = 1024 * 1024 // 1MB chunks for testing
}

// setupMinIOContainer sets up a containerized MinIO instance for isolated testing
func (te *TestEnvironment) setupMinIOContainer() error {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": "testuser",
			"MINIO_SECRET_KEY": "testpass123",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start MinIO container: %w", err)
	}

	te.MinIOContainer = container

	// Get container endpoint
	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get container endpoint: %w", err)
	}

	// Setup config for container
	te.Config = &config.Config{
		MinIOEndpoint:  endpoint,
		MinIOAccessKey: "testuser",
		MinIOSecretKey: "testpass123",
		MinIOSecure:    false,
	}

	te.TestBucket = "test-bucket-" + fmt.Sprintf("%d", time.Now().Unix())
	te.Config.MinioBucket = te.TestBucket
	te.Config.MaxConcurrentUploads = 10
	te.Config.ChunkSize = 1024 * 1024

	// Add cleanup for container
	te.Cleanup = append(te.Cleanup, func() {
		if te.MinIOContainer != nil {
			te.MinIOContainer.Terminate(context.Background())
		}
	})

	return nil
}

// Close cleans up the test environment
func (te *TestEnvironment) Close() {
	for _, cleanup := range te.Cleanup {
		cleanup()
	}
}

// TestFileGenerator provides utilities for generating test files
type TestFileGenerator struct {
	tempDir string
}

// NewTestFileGenerator creates a new test file generator
func NewTestFileGenerator(tempDir string) *TestFileGenerator {
	return &TestFileGenerator{
		tempDir: tempDir,
	}
}

// WAVTestFile represents a generated WAV test file
type WAVTestFile struct {
	Path       string
	Size       int64
	Hash       string
	Data       []byte
	Filename   string
	Pattern    string
	Duration   int // seconds
	SampleRate int
	BitDepth   int
	Channels   int
}

// GenerateWAVFile creates a test WAV file with specified characteristics
func (tfg *TestFileGenerator) GenerateWAVFile(spec WAVFileSpec) (*WAVTestFile, error) {
	// Calculate file parameters
	bytesPerSample := spec.BitDepth / 8
	numSamples := spec.Duration * spec.SampleRate * spec.Channels
	dataSize := numSamples * bytesPerSample

	// Create WAV header
	header := WAVHeader{
		RIFFHeader:    [4]byte{'R', 'I', 'F', 'F'},
		FileSize:      uint32(36 + dataSize),
		WAVEHeader:    [4]byte{'W', 'A', 'V', 'E'},
		FMTHeader:     [4]byte{'f', 'm', 't', ' '},
		FMTSize:       16,
		AudioFormat:   1, // PCM
		NumChannels:   uint16(spec.Channels),
		SampleRate:    uint32(spec.SampleRate),
		ByteRate:      uint32(spec.SampleRate * spec.Channels * bytesPerSample),
		BlockAlign:    uint16(spec.Channels * bytesPerSample),
		BitsPerSample: uint16(spec.BitDepth),
		DataHeader:    [4]byte{'d', 'a', 't', 'a'},
		DataSize:      uint32(dataSize),
	}

	// Create buffer for complete file
	var buf bytes.Buffer

	// Write header
	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return nil, fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Generate audio data
	if err := tfg.generateAudioData(&buf, spec, numSamples); err != nil {
		return nil, fmt.Errorf("failed to generate audio data: %w", err)
	}

	// Get complete file data
	fileData := buf.Bytes()
	hash := fmt.Sprintf("%x", sha256.Sum256(fileData))

	// Write to file if temp dir is available
	var filePath string
	if tfg.tempDir != "" {
		filePath = filepath.Join(tfg.tempDir, spec.Filename)
		if err := os.WriteFile(filePath, fileData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write test file: %w", err)
		}
	}

	return &WAVTestFile{
		Path:       filePath,
		Size:       int64(len(fileData)),
		Hash:       hash,
		Data:       fileData,
		Filename:   spec.Filename,
		Pattern:    spec.Pattern,
		Duration:   spec.Duration,
		SampleRate: spec.SampleRate,
		BitDepth:   spec.BitDepth,
		Channels:   spec.Channels,
	}, nil
}

// WAVFileSpec defines the characteristics of a WAV file to generate
type WAVFileSpec struct {
	Filename   string
	Duration   int // seconds
	SampleRate int // Hz
	BitDepth   int // bits
	Channels   int
	Pattern    string // "sine", "silence", "noise", "predictable"
}

// WAVHeader represents a WAV file header
type WAVHeader struct {
	// RIFF Header
	RIFFHeader [4]byte // "RIFF"
	FileSize   uint32  // File size - 8 bytes
	WAVEHeader [4]byte // "WAVE"

	// Format Chunk
	FMTHeader     [4]byte // "fmt "
	FMTSize       uint32  // Format chunk size (16 for PCM)
	AudioFormat   uint16  // Audio format (1 = PCM)
	NumChannels   uint16  // Number of channels
	SampleRate    uint32  // Sample rate
	ByteRate      uint32  // Byte rate
	BlockAlign    uint16  // Block alignment
	BitsPerSample uint16  // Bits per sample

	// Data Chunk
	DataHeader [4]byte // "data"
	DataSize   uint32  // Size of data section
}

// generateAudioData generates audio samples based on the specified pattern
func (tfg *TestFileGenerator) generateAudioData(writer io.Writer, spec WAVFileSpec, numSamples int) error {
	switch spec.Pattern {
	case "sine":
		return tfg.generateSineWave(writer, spec, numSamples)
	case "silence":
		return tfg.generateSilence(writer, spec, numSamples)
	case "noise":
		return tfg.generateNoise(writer, spec, numSamples)
	case "predictable":
		return tfg.generatePredictableData(writer, spec, numSamples)
	default:
		return tfg.generatePredictableData(writer, spec, numSamples)
	}
}

// generateSineWave generates a sine wave pattern
func (tfg *TestFileGenerator) generateSineWave(writer io.Writer, spec WAVFileSpec, numSamples int) error {
	frequency := 440.0 // A4 note
	amplitude := float64((1<<(spec.BitDepth-1))-1) * 0.8

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(spec.SampleRate*spec.Channels)
		value := amplitude * math.Sin(2*math.Pi*frequency*t)

		if err := tfg.writeSample(writer, int32(value), spec.BitDepth); err != nil {
			return err
		}
	}

	return nil
}

// generateSilence generates silence (all zeros)
func (tfg *TestFileGenerator) generateSilence(writer io.Writer, spec WAVFileSpec, numSamples int) error {
	for i := 0; i < numSamples; i++ {
		if err := tfg.writeSample(writer, 0, spec.BitDepth); err != nil {
			return err
		}
	}
	return nil
}

// generateNoise generates random noise
func (tfg *TestFileGenerator) generateNoise(writer io.Writer, spec WAVFileSpec, numSamples int) error {
	maxValue := int32((1 << (spec.BitDepth - 1)) - 1)

	for i := 0; i < numSamples; i++ {
		var value int32
		if err := binary.Read(rand.Reader, binary.LittleEndian, &value); err != nil {
			return err
		}

		value = value % maxValue

		if err := tfg.writeSample(writer, value, spec.BitDepth); err != nil {
			return err
		}
	}

	return nil
}

// generatePredictableData generates predictable, repeating patterns for hash consistency
func (tfg *TestFileGenerator) generatePredictableData(writer io.Writer, spec WAVFileSpec, numSamples int) error {
	maxValue := int32((1 << (spec.BitDepth - 1)) - 1)

	for i := 0; i < numSamples; i++ {
		value := int32((i % 1000) - 500) // Simple sawtooth pattern
		value = value * maxValue / 500   // Scale to bit depth

		if err := tfg.writeSample(writer, value, spec.BitDepth); err != nil {
			return err
		}
	}

	return nil
}

// writeSample writes a single audio sample to the writer
func (tfg *TestFileGenerator) writeSample(writer io.Writer, value int32, bitDepth int) error {
	switch bitDepth {
	case 16:
		return binary.Write(writer, binary.LittleEndian, int16(value))
	case 24:
		// 24-bit is stored as 3 bytes in little-endian format
		bytes := []byte{
			byte(value & 0xFF),
			byte((value >> 8) & 0xFF),
			byte((value >> 16) & 0xFF),
		}
		_, err := writer.Write(bytes)
		return err
	case 32:
		return binary.Write(writer, binary.LittleEndian, value)
	default:
		return fmt.Errorf("unsupported bit depth: %d", bitDepth)
	}
}

// GenerateTestSuite generates a comprehensive test suite with various file types
func (tfg *TestFileGenerator) GenerateTestSuite() ([]*WAVTestFile, error) {
	specs := []WAVFileSpec{
		// Small files for basic testing
		{"test_small_mono.wav", 5, 22050, 16, 1, "predictable"},
		{"test_small_stereo.wav", 5, 44100, 16, 2, "predictable"},

		// Medium files for performance testing
		{"test_medium_cd.wav", 30, 44100, 16, 2, "predictable"},
		{"test_medium_hd.wav", 30, 48000, 24, 2, "predictable"},

		// Large files for stress testing
		{"test_large_cd.wav", 300, 44100, 16, 2, "predictable"}, // ~50MB
		{"test_large_hd.wav", 120, 96000, 24, 2, "predictable"}, // ~70MB

		// Different audio patterns
		{"test_sine_wave.wav", 10, 44100, 16, 2, "sine"},
		{"test_silence.wav", 10, 44100, 16, 2, "silence"},
		{"test_noise.wav", 10, 44100, 16, 2, "noise"},

		// Edge cases
		{"test_mono_8khz.wav", 10, 8000, 16, 1, "predictable"},    // Low sample rate
		{"test_stereo_192k.wav", 5, 192000, 24, 2, "predictable"}, // High sample rate
	}

	var testFiles []*WAVTestFile

	for _, spec := range specs {
		testFile, err := tfg.GenerateWAVFile(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to generate %s: %w", spec.Filename, err)
		}
		testFiles = append(testFiles, testFile)
	}

	return testFiles, nil
}

// TestDataGenerator provides utilities for generating various test data
type TestDataGenerator struct {
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{}
}

// GenerateRandomData generates random data of specified size
func (tdg *TestDataGenerator) GenerateRandomData(size int64) ([]byte, string) {
	data := make([]byte, size)
	rand.Read(data)
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return data, hash
}

// GeneratePredictableData generates predictable data for consistent testing
func (tdg *TestDataGenerator) GeneratePredictableData(size int64, seed int64) ([]byte, string) {
	data := make([]byte, size)

	for i := int64(0); i < size; i++ {
		data[i] = byte((i + seed) % 256)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return data, hash
}

// GenerateZeroData generates zero-filled data
func (tdg *TestDataGenerator) GenerateZeroData(size int64) ([]byte, string) {
	data := make([]byte, size)
	// Data is already zero-filled
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return data, hash
}

// TestMetricsCollector collects and tracks test metrics
type TestMetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]interface{}
	start   time.Time
}

// NewTestMetricsCollector creates a new metrics collector
func NewTestMetricsCollector() *TestMetricsCollector {
	return &TestMetricsCollector{
		metrics: make(map[string]interface{}),
		start:   time.Now(),
	}
}

// Record records a metric value
func (tmc *TestMetricsCollector) Record(key string, value interface{}) {
	tmc.mu.Lock()
	defer tmc.mu.Unlock()
	tmc.metrics[key] = value
}

// Increment increments a counter metric
func (tmc *TestMetricsCollector) Increment(key string) {
	tmc.mu.Lock()
	defer tmc.mu.Unlock()

	if val, exists := tmc.metrics[key]; exists {
		if counter, ok := val.(int64); ok {
			tmc.metrics[key] = counter + 1
		}
	} else {
		tmc.metrics[key] = int64(1)
	}
}

// Get retrieves a metric value
func (tmc *TestMetricsCollector) Get(key string) (interface{}, bool) {
	tmc.mu.RLock()
	defer tmc.mu.RUnlock()
	val, exists := tmc.metrics[key]
	return val, exists
}

// GetAll returns all metrics
func (tmc *TestMetricsCollector) GetAll() map[string]interface{} {
	tmc.mu.RLock()
	defer tmc.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range tmc.metrics {
		result[k] = v
	}

	result["test_duration"] = time.Since(tmc.start)
	return result
}

// TestResourceMonitor monitors system resources during tests
type TestResourceMonitor struct {
	mu           sync.RWMutex
	monitoring   bool
	stopChan     chan struct{}
	measurements []ResourceMeasurement
}

// ResourceMeasurement represents a point-in-time resource measurement
type ResourceMeasurement struct {
	Timestamp  time.Time `json:"timestamp"`
	CPUPercent float64   `json:"cpu_percent"`
	MemoryMB   uint64    `json:"memory_mb"`
	Goroutines int       `json:"goroutines"`
	CGoCalls   int64     `json:"cgo_calls"`
}

// NewTestResourceMonitor creates a new resource monitor
func NewTestResourceMonitor() *TestResourceMonitor {
	return &TestResourceMonitor{
		stopChan:     make(chan struct{}),
		measurements: make([]ResourceMeasurement, 0),
	}
}

// Start begins resource monitoring
func (trm *TestResourceMonitor) Start(interval time.Duration) {
	trm.mu.Lock()
	defer trm.mu.Unlock()

	if trm.monitoring {
		return
	}

	trm.monitoring = true

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-trm.stopChan:
				return
			case <-ticker.C:
				measurement := ResourceMeasurement{
					Timestamp:  time.Now(),
					Goroutines: runtime.NumGoroutine(),
					CGoCalls:   runtime.NumCgoCall(),
				}

				trm.mu.Lock()
				trm.measurements = append(trm.measurements, measurement)
				trm.mu.Unlock()
			}
		}
	}()
}

// Stop stops resource monitoring
func (trm *TestResourceMonitor) Stop() {
	trm.mu.Lock()
	defer trm.mu.Unlock()

	if !trm.monitoring {
		return
	}

	close(trm.stopChan)
	trm.monitoring = false
	trm.stopChan = make(chan struct{}) // Reset for potential restart
}

// GetPeakUsage returns peak resource usage during monitoring
func (trm *TestResourceMonitor) GetPeakUsage() ResourceMeasurement {
	trm.mu.RLock()
	defer trm.mu.RUnlock()

	if len(trm.measurements) == 0 {
		return ResourceMeasurement{}
	}

	peak := trm.measurements[0]
	for _, measurement := range trm.measurements {
		if measurement.CPUPercent > peak.CPUPercent {
			peak.CPUPercent = measurement.CPUPercent
		}
		if measurement.MemoryMB > peak.MemoryMB {
			peak.MemoryMB = measurement.MemoryMB
		}
		if measurement.Goroutines > peak.Goroutines {
			peak.Goroutines = measurement.Goroutines
		}
		if measurement.CGoCalls > peak.CGoCalls {
			peak.CGoCalls = measurement.CGoCalls
		}
	}

	return peak
}

// GetAllMeasurements returns all resource measurements
func (trm *TestResourceMonitor) GetAllMeasurements() []ResourceMeasurement {
	trm.mu.RLock()
	defer trm.mu.RUnlock()

	result := make([]ResourceMeasurement, len(trm.measurements))
	copy(result, trm.measurements)
	return result
}

// MinIOTestHelper provides utilities for MinIO testing
type MinIOTestHelper struct {
	service *services.MinIOService
	bucket  string
}

// NewMinIOTestHelper creates a new MinIO test helper
func NewMinIOTestHelper(service *services.MinIOService, bucket string) *MinIOTestHelper {
	return &MinIOTestHelper{
		service: service,
		bucket:  bucket,
	}
}

// UploadTestFile uploads a test file and returns metadata
func (mth *MinIOTestHelper) UploadTestFile(testFile *WAVTestFile) (*services.FileMetadata, error) {
	reader := bytes.NewReader(testFile.Data)
	return mth.service.UploadFileStreaming(reader, testFile.Filename, testFile.Size, testFile.Hash)
}

// VerifyFileExists checks if a file exists in MinIO
func (mth *MinIOTestHelper) VerifyFileExists(filename string) (bool, error) {
	ctx := context.Background()
	_, err := mth.service.GetClient().StatObject(ctx, mth.bucket, filename, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// VerifyFileIntegrity verifies file integrity by comparing hashes
func (mth *MinIOTestHelper) VerifyFileIntegrity(filename, expectedHash string) (*services.IntegrityResult, error) {
	return mth.service.VerifyUploadIntegrity(filename, expectedHash)
}

// ListTestFiles lists all files with test prefix
func (mth *MinIOTestHelper) ListTestFiles(prefix string) ([]minio.ObjectInfo, error) {
	ctx := context.Background()
	var objects []minio.ObjectInfo

	objectCh := mth.service.GetClient().ListObjects(ctx, mth.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, object)
	}

	return objects, nil
}

// CleanupTestFiles removes all test files from the bucket
func (mth *MinIOTestHelper) CleanupTestFiles(prefix string) error {
	objects, err := mth.ListTestFiles(prefix)
	if err != nil {
		return err
	}

	ctx := context.Background()
	for _, obj := range objects {
		err = mth.service.GetClient().RemoveObject(ctx, mth.bucket, obj.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// ConcurrencyTester provides utilities for testing concurrent operations
type ConcurrencyTester struct {
	goroutineLimit int
	semaphore      chan struct{}
}

// NewConcurrencyTester creates a new concurrency tester with specified limit
func NewConcurrencyTester(maxGoroutines int) *ConcurrencyTester {
	return &ConcurrencyTester{
		goroutineLimit: maxGoroutines,
		semaphore:      make(chan struct{}, maxGoroutines),
	}
}

// RunConcurrent runs functions concurrently with goroutine limiting
func (ct *ConcurrencyTester) RunConcurrent(functions []func() error) []error {
	results := make([]error, len(functions))
	var wg sync.WaitGroup

	for i, fn := range functions {
		wg.Add(1)
		go func(index int, function func() error) {
			defer wg.Done()

			// Acquire semaphore
			ct.semaphore <- struct{}{}
			defer func() { <-ct.semaphore }()

			results[index] = function()
		}(i, fn)
	}

	wg.Wait()
	return results
}

// RunConcurrentWithTimeout runs functions concurrently with timeout
func (ct *ConcurrencyTester) RunConcurrentWithTimeout(functions []func() error, timeout time.Duration) ([]error, error) {
	results := make([]error, len(functions))
	var wg sync.WaitGroup
	done := make(chan struct{})

	go func() {
		for i, fn := range functions {
			wg.Add(1)
			go func(index int, function func() error) {
				defer wg.Done()

				ct.semaphore <- struct{}{}
				defer func() { <-ct.semaphore }()

				results[index] = function()
			}(i, fn)
		}
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return results, nil
	case <-time.After(timeout):
		return results, fmt.Errorf("concurrent operations timed out after %v", timeout)
	}
}

// Default test file specs for common use cases
var (
	SmallTestFileSpecs = []WAVFileSpec{
		{"small_mono.wav", 5, 22050, 16, 1, "predictable"},
		{"small_stereo.wav", 5, 44100, 16, 2, "predictable"},
	}

	MediumTestFileSpecs = []WAVFileSpec{
		{"medium_cd.wav", 30, 44100, 16, 2, "predictable"},
		{"medium_hd.wav", 30, 48000, 24, 2, "predictable"},
	}

	LargeTestFileSpecs = []WAVFileSpec{
		{"large_cd.wav", 300, 44100, 16, 2, "predictable"},
		{"large_hd.wav", 120, 96000, 24, 2, "predictable"},
	}

	StressTestFileSpecs = []WAVFileSpec{
		{"stress_1.wav", 10, 44100, 16, 2, "predictable"},
		{"stress_2.wav", 10, 44100, 16, 2, "predictable"},
		{"stress_3.wav", 10, 44100, 16, 2, "predictable"},
		{"stress_4.wav", 10, 44100, 16, 2, "predictable"},
		{"stress_5.wav", 10, 44100, 16, 2, "predictable"},
	}
)
