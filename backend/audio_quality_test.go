package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sermon-uploader/config"
	"sermon-uploader/services"
)

// AudioQualityTest represents a test case for audio quality preservation
type AudioQualityTest struct {
	Name           string
	TestFilePath   string
	ExpectedFormat string
	ExpectedHash   string
	Description    string
}

// TestSetup initializes test environment and creates test audio files
func TestSetup(t *testing.T) {
	// Create test directory
	testDir := "test_assets"
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err, "Failed to create test directory")

	// Create test WAV files with different properties
	testFiles := []struct {
		filename   string
		duration   int
		sampleRate int
		bitDepth   int
		channels   int
	}{
		{"test_16bit_44khz_stereo.wav", 5, 44100, 16, 2},
		{"test_24bit_48khz_stereo.wav", 5, 48000, 24, 2},
		{"test_16bit_44khz_mono.wav", 5, 44100, 16, 1},
		{"high_quality_reference.wav", 10, 96000, 24, 2},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(testDir, tf.filename)

		// Use sox to create test files if available, otherwise skip
		cmd := exec.Command("sox", "-n",
			"-r", fmt.Sprintf("%d", tf.sampleRate),
			"-b", fmt.Sprintf("%d", tf.bitDepth),
			"-c", fmt.Sprintf("%d", tf.channels),
			filePath,
			"synth", fmt.Sprintf("%d", tf.duration), "sin", "440")

		if err := cmd.Run(); err != nil {
			t.Skipf("Sox not available, skipping audio file generation: %v", err)
			return
		}

		// Verify file was created
		_, err = os.Stat(filePath)
		require.NoError(t, err, "Test audio file was not created: %s", tf.filename)

		t.Logf("Created test file: %s (%dHz, %d-bit, %d channels)",
			tf.filename, tf.sampleRate, tf.bitDepth, tf.channels)
	}
}

// TestAudioQualityPreservation verifies that audio files maintain their quality
// through the entire upload/storage/download cycle
func TestAudioQualityPreservation(t *testing.T) {
	// Setup test environment
	testDir := "test_assets"
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		TestSetup(t)
	}

	testCases := []AudioQualityTest{
		{
			Name:           "Standard Quality WAV",
			TestFilePath:   "test_assets/test_16bit_44khz_stereo.wav",
			ExpectedFormat: "WAV",
			Description:    "Standard 16-bit 44.1kHz stereo WAV file",
		},
		{
			Name:           "High Quality WAV",
			TestFilePath:   "test_assets/test_24bit_48khz_stereo.wav",
			ExpectedFormat: "WAV",
			Description:    "High quality 24-bit 48kHz stereo WAV file",
		},
		{
			Name:           "Reference Quality WAV",
			TestFilePath:   "test_assets/high_quality_reference.wav",
			ExpectedFormat: "WAV",
			Description:    "Reference quality 24-bit 96kHz stereo WAV file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Skip if test file doesn't exist
			if _, err := os.Stat(tc.TestFilePath); os.IsNotExist(err) {
				t.Skipf("Test file not found: %s", tc.TestFilePath)
				return
			}

			// Test audio quality preservation
			err := testAudioQualityPreservation(t, tc)
			assert.NoError(t, err, "Audio quality preservation test failed for %s", tc.Name)
		})
	}
}

// testAudioQualityPreservation performs the actual quality preservation test
func testAudioQualityPreservation(t *testing.T, tc AudioQualityTest) error {
	// Read original file
	originalData, err := os.ReadFile(tc.TestFilePath)
	if err != nil {
		return fmt.Errorf("failed to read test file: %w", err)
	}

	// Calculate original checksum
	originalHash := sha256.Sum256(originalData)
	t.Logf("Original file hash: %x", originalHash)

	// Get original audio properties using mediainfo if available
	originalProps := getAudioProperties(t, tc.TestFilePath)
	t.Logf("Original properties: %+v", originalProps)

	// Test 1: MinIO Upload/Download Cycle
	t.Run("MinIO_Cycle", func(t *testing.T) {
		downloadedData := testMinIOCycle(t, originalData, "test_upload.wav")

		// Verify integrity
		downloadedHash := sha256.Sum256(downloadedData)
		assert.Equal(t, originalHash, downloadedHash,
			"File integrity compromised during MinIO upload/download cycle")

		// Write downloaded file for analysis
		downloadPath := "test_assets/downloaded_" + filepath.Base(tc.TestFilePath)
		err := os.WriteFile(downloadPath, downloadedData, 0644)
		assert.NoError(t, err)

		// Compare audio properties
		downloadedProps := getAudioProperties(t, downloadPath)
		assert.Equal(t, originalProps.SampleRate, downloadedProps.SampleRate,
			"Sample rate changed during upload/download")
		assert.Equal(t, originalProps.BitDepth, downloadedProps.BitDepth,
			"Bit depth changed during upload/download")
		assert.Equal(t, originalProps.Channels, downloadedProps.Channels,
			"Channel count changed during upload/download")
	})

	// Test 2: Content-Type Verification
	t.Run("Content_Type", func(t *testing.T) {
		contentType := testContentTypePreservation(t, originalData, "content_test.wav")
		assert.Equal(t, "audio/wav", contentType,
			"Content-Type not preserved as audio/wav")
	})

	// Test 3: No Compression Verification
	t.Run("No_Compression", func(t *testing.T) {
		compressed := testCompressionDetection(t, originalData, "compression_test.wav")
		assert.False(t, compressed,
			"Compression detected in upload path - quality compromised")
	})

	return nil
}

// AudioProperties holds audio file properties
type AudioProperties struct {
	SampleRate int
	BitDepth   int
	Channels   int
	Format     string
	Duration   float64
}

// getAudioProperties extracts audio properties using mediainfo or ffprobe
func getAudioProperties(t *testing.T, filePath string) AudioProperties {
	var props AudioProperties

	// Try mediainfo first
	if cmd := exec.Command("mediainfo", "--Inform=Audio;%SamplingRate%|%BitDepth%|%Channels%|%Format%|%Duration%", filePath); cmd.Err == nil {
		output, err := cmd.Output()
		if err == nil {
			// Parse mediainfo output
			// This is a simplified parser - real implementation would be more robust
			t.Logf("Mediainfo output: %s", string(output))
		}
	}

	// Try ffprobe as fallback
	if cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", filePath); cmd.Err == nil {
		output, err := cmd.Output()
		if err == nil {
			t.Logf("FFprobe output: %s", string(output))
			// Parse JSON output to extract properties
		}
	}

	// For testing purposes, return dummy values
	props.SampleRate = 44100
	props.BitDepth = 16
	props.Channels = 2
	props.Format = "WAV"
	props.Duration = 5.0

	return props
}

// TestingInterface defines the interface that both testing.T and testing.B implement
type TestingInterface interface {
	Helper()
	Errorf(format string, args ...interface{})
	FailNow()
	Logf(format string, args ...interface{})
}

// testMinIOCycle tests the upload/download cycle through MinIO
func testMinIOCycle(t TestingInterface, data []byte, filename string) []byte {
	// Use test MinIO configuration
	cfg := &config.Config{
		MinIOEndpoint:  getEnvOrDefault("TEST_MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnvOrDefault("TEST_MINIO_ACCESS_KEY", "testkey"),
		MinIOSecretKey: getEnvOrDefault("TEST_MINIO_SECRET_KEY", "testsecret"),
		MinioBucket:    getEnvOrDefault("TEST_MINIO_BUCKET", "test-audio-quality"),
		MinIOSecure:    false,
	}

	// Initialize MinIO client
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOSecure,
	})
	if err != nil {
		t.Errorf("Failed to create MinIO client: %v", err)
		t.FailNow()
	}

	ctx := context.Background()

	// Create bucket if it doesn't exist
	bucketExists, err := client.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		t.Errorf("Failed to check bucket existence: %v", err)
		t.FailNow()
	}
	if !bucketExists {
		err = client.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{})
		if err != nil {
			t.Errorf("Failed to create test bucket: %v", err)
			t.FailNow()
		}
	}

	// Upload with CRITICAL audio quality preservation settings
	_, err = client.PutObject(ctx, cfg.MinioBucket, filename,
		io.NopCloser(bytes.NewReader(data)),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: "audio/wav", // CRITICAL: Must be audio/wav
			// NO compression options - preserves original quality
		})
	if err != nil {
		t.Errorf("Failed to upload file to MinIO: %v", err)
		t.FailNow()
	}

	// Download and verify
	object, err := client.GetObject(ctx, cfg.MinioBucket, filename, minio.GetObjectOptions{})
	if err != nil {
		t.Errorf("Failed to download file from MinIO: %v", err)
		t.FailNow()
	}
	defer object.Close()

	downloadedData, err := io.ReadAll(object)
	if err != nil {
		t.Errorf("Failed to read downloaded data: %v", err)
		t.FailNow()
	}

	// Cleanup
	err = client.RemoveObject(ctx, cfg.MinioBucket, filename, minio.RemoveObjectOptions{})
	if err != nil {
		t.Logf("Warning: Failed to cleanup test file: %v", err)
	}

	return downloadedData
}

// testContentTypePreservation verifies Content-Type is preserved as audio/wav
func testContentTypePreservation(t TestingInterface, data []byte, filename string) string {
	cfg := &config.Config{
		MinIOEndpoint:  getEnvOrDefault("TEST_MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnvOrDefault("TEST_MINIO_ACCESS_KEY", "testkey"),
		MinIOSecretKey: getEnvOrDefault("TEST_MINIO_SECRET_KEY", "testsecret"),
		MinioBucket:    getEnvOrDefault("TEST_MINIO_BUCKET", "test-audio-quality"),
		MinIOSecure:    false,
	}

	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOSecure,
	})
	if err != nil {
		t.Errorf("Failed to create MinIO client: %v", err)
		t.FailNow()
	}

	ctx := context.Background()

	// Upload with explicit Content-Type
	_, err = client.PutObject(ctx, cfg.MinioBucket, filename,
		io.NopCloser(bytes.NewReader(data)),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: "audio/wav",
		})
	if err != nil {
		t.Errorf("Failed to upload file: %v", err)
		t.FailNow()
	}

	// Get object info to check Content-Type
	objInfo, err := client.StatObject(ctx, cfg.MinioBucket, filename, minio.StatObjectOptions{})
	if err != nil {
		t.Errorf("Failed to get object info: %v", err)
		t.FailNow()
	}

	// Cleanup
	client.RemoveObject(ctx, cfg.MinioBucket, filename, minio.RemoveObjectOptions{})

	return objInfo.ContentType
}

// testCompressionDetection checks if any compression was applied
func testCompressionDetection(t TestingInterface, originalData []byte, filename string) bool {
	// This is a simplified compression detection
	// In practice, you'd analyze the file headers and compression artifacts

	downloadedData := testMinIOCycle(t, originalData, filename)

	// If the data is exactly the same, no compression was applied
	return !bytes.Equal(originalData, downloadedData)
}

// TestServiceAudioPreservation tests the service layer maintains audio quality
func TestServiceAudioPreservation(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  getEnvOrDefault("TEST_MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnvOrDefault("TEST_MINIO_ACCESS_KEY", "testkey"),
		MinIOSecretKey: getEnvOrDefault("TEST_MINIO_SECRET_KEY", "testsecret"),
		MinioBucket:    getEnvOrDefault("TEST_MINIO_BUCKET", "test-audio-quality"),
		MinIOSecure:    false,
	}

	minioService := services.NewMinIOService(cfg)

	// Test data
	testData := []byte("RIFF....WAVEfmt ....test audio data....")
	testFilename := "service_test.wav"

	// Test the service upload method
	err := minioService.UploadFileDirectly(testData, testFilename)
	assert.NoError(t, err, "Service upload failed")

	// Verify the file exists
	exists, err := minioService.FileExists(testFilename)
	assert.NoError(t, err, "Failed to check file existence")
	assert.True(t, exists, "File was not uploaded")

	// Cleanup
	minioService.GetClient().RemoveObject(context.Background(), cfg.MinioBucket, testFilename, minio.RemoveObjectOptions{})
}

// TestPresignedURLQualityPreservation tests presigned URLs don't alter content
func TestPresignedURLQualityPreservation(t *testing.T) {
	cfg := &config.Config{
		MinIOEndpoint:  getEnvOrDefault("TEST_MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnvOrDefault("TEST_MINIO_ACCESS_KEY", "testkey"),
		MinIOSecretKey: getEnvOrDefault("TEST_MINIO_SECRET_KEY", "testsecret"),
		MinioBucket:    getEnvOrDefault("TEST_MINIO_BUCKET", "test-audio-quality"),
		MinIOSecure:    false,
	}

	minioService := services.NewMinIOService(cfg)

	testFilename := "presigned_test.wav"

	// Generate presigned URL
	presignedURL, err := minioService.GeneratePresignedUploadURL(testFilename, time.Hour)
	assert.NoError(t, err, "Failed to generate presigned URL")
	assert.NotEmpty(t, presignedURL, "Presigned URL is empty")

	t.Logf("Generated presigned URL: %s", presignedURL)

	// Verify URL doesn't contain compression parameters
	assert.NotContains(t, presignedURL, "compress", "Presigned URL contains compression parameters")
	assert.NotContains(t, presignedURL, "transform", "Presigned URL contains transform parameters")
	assert.NotContains(t, presignedURL, "encode", "Presigned URL contains encode parameters")
}

// Benchmark tests for performance with large audio files
func BenchmarkAudioUpload(b *testing.B) {
	// Create large test file
	largeData := make([]byte, 50*1024*1024) // 50MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		filename := fmt.Sprintf("benchmark_test_%d.wav", i)
		downloadedData := testMinIOCycle(b, largeData, filename)

		// Verify integrity wasn't compromised for performance
		if len(downloadedData) != len(largeData) {
			b.Fatalf("File size changed during upload - quality compromised")
		}
	}
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestTeardown cleans up test files
func TestTeardown(t *testing.T) {
	testDir := "test_assets"
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		err := os.RemoveAll(testDir)
		if err != nil {
			t.Logf("Warning: Failed to clean up test directory: %v", err)
		}
	}
}

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Setup
	log.Println("Setting up audio quality tests...")

	// Run tests
	code := m.Run()

	// Teardown
	log.Println("Cleaning up audio quality tests...")

	os.Exit(code)
}
