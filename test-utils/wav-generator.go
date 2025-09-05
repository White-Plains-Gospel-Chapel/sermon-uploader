package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"
)

// WAVGenerator creates test WAV files with specific characteristics for audio preservation testing
type WAVGenerator struct {
	OutputDir string
}

// WAVSpec defines the characteristics of a WAV file to generate
type WAVSpec struct {
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
	RIFFHeader    [4]byte // "RIFF"
	FileSize      uint32  // File size - 8 bytes
	WAVEHeader    [4]byte // "WAVE"
	
	// Format Chunk
	FMTHeader     [4]byte // "fmt "
	FMTSize       uint32  // Format chunk size (16 for PCM)
	AudioFormat   uint16  // Audio format (1 = PCM)
	NumChannels   uint16  // Number of channels
	SampleRate    uint32  // Sample rate
	ByteRate      uint32  // Byte rate (SampleRate * NumChannels * BitsPerSample/8)
	BlockAlign    uint16  // Block alignment (NumChannels * BitsPerSample/8)
	BitsPerSample uint16  // Bits per sample
	
	// Data Chunk
	DataHeader    [4]byte // "data"
	DataSize      uint32  // Size of data section
}

// NewWAVGenerator creates a new WAV generator
func NewWAVGenerator(outputDir string) *WAVGenerator {
	return &WAVGenerator{
		OutputDir: outputDir,
	}
}

// GenerateWAV creates a WAV file with the specified characteristics
func (g *WAVGenerator) GenerateWAV(spec WAVSpec) (string, string, error) {
	// Calculate file parameters
	bytesPerSample := spec.BitDepth / 8
	numSamples := spec.Duration * spec.SampleRate * spec.Channels
	dataSize := numSamples * bytesPerSample
	fileSize := 36 + dataSize // 44 byte header - 8 bytes for RIFF header
	
	// Create WAV header
	header := WAVHeader{
		RIFFHeader:    [4]byte{'R', 'I', 'F', 'F'},
		FileSize:      uint32(fileSize),
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
	
	// Create output file
	outputPath := filepath.Join(g.OutputDir, spec.Filename)
	file, err := os.Create(outputPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()
	
	// Create hash calculator
	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)
	
	// Write header
	if err := binary.Write(writer, binary.LittleEndian, header); err != nil {
		return "", "", fmt.Errorf("failed to write header: %w", err)
	}
	
	// Generate and write audio data
	if err := g.generateAudioData(writer, spec, numSamples); err != nil {
		return "", "", fmt.Errorf("failed to generate audio data: %w", err)
	}
	
	// Calculate file hash
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	
	return outputPath, hash, nil
}

// generateAudioData generates audio samples based on the specified pattern
func (g *WAVGenerator) generateAudioData(writer io.Writer, spec WAVSpec, numSamples int) error {
	switch spec.Pattern {
	case "sine":
		return g.generateSineWave(writer, spec, numSamples)
	case "silence":
		return g.generateSilence(writer, spec, numSamples)
	case "noise":
		return g.generateNoise(writer, spec, numSamples)
	case "predictable":
		return g.generatePredictableData(writer, spec, numSamples)
	default:
		return g.generatePredictableData(writer, spec, numSamples)
	}
}

// generateSineWave generates a sine wave pattern
func (g *WAVGenerator) generateSineWave(writer io.Writer, spec WAVSpec, numSamples int) error {
	frequency := 440.0 // A4 note
	amplitude := float64((1 << (spec.BitDepth - 1)) - 1) * 0.8 // 80% of max amplitude
	
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(spec.SampleRate * spec.Channels)
		value := amplitude * math.Sin(2*math.Pi*frequency*t)
		
		if err := g.writeSample(writer, int32(value), spec.BitDepth); err != nil {
			return err
		}
	}
	
	return nil
}

// generateSilence generates silence (all zeros)
func (g *WAVGenerator) generateSilence(writer io.Writer, spec WAVSpec, numSamples int) error {
	for i := 0; i < numSamples; i++ {
		if err := g.writeSample(writer, 0, spec.BitDepth); err != nil {
			return err
		}
	}
	return nil
}

// generateNoise generates random noise
func (g *WAVGenerator) generateNoise(writer io.Writer, spec WAVSpec, numSamples int) error {
	maxValue := int32((1 << (spec.BitDepth - 1)) - 1)
	
	for i := 0; i < numSamples; i++ {
		// Generate random value
		var value int32
		if err := binary.Read(rand.Reader, binary.LittleEndian, &value); err != nil {
			return err
		}
		
		// Scale to bit depth
		value = value % maxValue
		
		if err := g.writeSample(writer, value, spec.BitDepth); err != nil {
			return err
		}
	}
	
	return nil
}

// generatePredictableData generates predictable, repeating patterns for hash consistency
func (g *WAVGenerator) generatePredictableData(writer io.Writer, spec WAVSpec, numSamples int) error {
	maxValue := int32((1 << (spec.BitDepth - 1)) - 1)
	
	for i := 0; i < numSamples; i++ {
		// Create predictable pattern based on sample index
		value := int32((i % 1000) - 500) // Simple sawtooth pattern
		value = value * maxValue / 500   // Scale to bit depth
		
		if err := g.writeSample(writer, value, spec.BitDepth); err != nil {
			return err
		}
	}
	
	return nil
}

// writeSample writes a single audio sample to the writer
func (g *WAVGenerator) writeSample(writer io.Writer, value int32, bitDepth int) error {
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

// Benchmark represents upload performance metrics
type Benchmark struct {
	FileSize     int64         // bytes
	Duration     time.Duration // upload time
	Throughput   float64       // MB/s
	Success      bool
	Error        string
	Hash         string
	Integrity    bool // hash verification passed
}

// BenchmarkSuite runs comprehensive benchmarks
type BenchmarkSuite struct {
	generator *WAVGenerator
	results   []Benchmark
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(outputDir string) *BenchmarkSuite {
	return &BenchmarkSuite{
		generator: NewWAVGenerator(outputDir),
		results:   make([]Benchmark, 0),
	}
}

// RunBenchmarks executes the benchmark suite
func (bs *BenchmarkSuite) RunBenchmarks() error {
	fmt.Println("Starting WAV file generation and benchmark suite...")
	
	// Define test cases for different file sizes and qualities
	testCases := []WAVSpec{
		{"small_16bit_mono.wav", 30, 44100, 16, 1, "predictable"},       // ~2.5MB
		{"small_16bit_stereo.wav", 30, 44100, 16, 2, "predictable"},     // ~5MB
		{"medium_24bit_stereo.wav", 120, 48000, 24, 2, "predictable"},   // ~34MB
		{"large_24bit_stereo.wav", 600, 96000, 24, 2, "predictable"},    // ~345MB
		{"xlarge_24bit_stereo.wav", 1800, 96000, 24, 2, "predictable"},  // ~1GB
		{"sine_test.wav", 60, 44100, 16, 2, "sine"},                     // Test with sine wave
		{"noise_test.wav", 60, 44100, 24, 2, "noise"},                   // Test with noise
		{"silence_test.wav", 60, 48000, 16, 2, "silence"},               // Test with silence
	}
	
	for _, spec := range testCases {
		fmt.Printf("\nGenerating: %s\n", spec.Filename)
		fmt.Printf("  Duration: %ds, Sample Rate: %dHz, Bit Depth: %d-bit, Channels: %d\n", 
			spec.Duration, spec.SampleRate, spec.BitDepth, spec.Channels)
		
		startTime := time.Now()
		
		filePath, hash, err := bs.generator.GenerateWAV(spec)
		
		duration := time.Since(startTime)
		
		benchmark := Benchmark{
			Hash: hash,
		}
		
		if err != nil {
			benchmark.Success = false
			benchmark.Error = err.Error()
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			// Get file size
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				benchmark.Success = false
				benchmark.Error = fmt.Sprintf("failed to stat file: %v", err)
			} else {
				benchmark.FileSize = fileInfo.Size()
				benchmark.Duration = duration
				benchmark.Throughput = float64(benchmark.FileSize) / (1024 * 1024) / duration.Seconds()
				benchmark.Success = true
				
				// Verify integrity by recalculating hash
				verifyHash, err := bs.calculateFileHash(filePath)
				if err != nil {
					benchmark.Integrity = false
					fmt.Printf("  WARNING: Hash verification failed: %v\n", err)
				} else {
					benchmark.Integrity = (hash == verifyHash)
					if !benchmark.Integrity {
						fmt.Printf("  ERROR: Hash mismatch! Generated: %s, Verified: %s\n", hash, verifyHash)
					}
				}
				
				fmt.Printf("  Generated: %s\n", filePath)
				fmt.Printf("  Size: %.2f MB\n", float64(benchmark.FileSize)/(1024*1024))
				fmt.Printf("  Generation Time: %v\n", duration)
				fmt.Printf("  Throughput: %.2f MB/s\n", benchmark.Throughput)
				fmt.Printf("  Hash: %s\n", hash)
				fmt.Printf("  Integrity: %v\n", benchmark.Integrity)
			}
		}
		
		bs.results = append(bs.results, benchmark)
	}
	
	bs.printSummary()
	return nil
}

// calculateFileHash calculates SHA256 hash of a file
func (bs *BenchmarkSuite) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// printSummary prints benchmark results
func (bs *BenchmarkSuite) printSummary() {
	fmt.Println("\n" + "="*80)
	fmt.Println("BENCHMARK SUMMARY")
	fmt.Println("="*80)
	
	successCount := 0
	var totalSize int64
	var totalDuration time.Duration
	integrityFailures := 0
	
	for _, result := range bs.results {
		if result.Success {
			successCount++
			totalSize += result.FileSize
			totalDuration += result.Duration
			
			if !result.Integrity {
				integrityFailures++
			}
		}
	}
	
	fmt.Printf("Files Generated: %d/%d successful\n", successCount, len(bs.results))
	fmt.Printf("Total Size: %.2f MB\n", float64(totalSize)/(1024*1024))
	fmt.Printf("Total Time: %v\n", totalDuration)
	fmt.Printf("Average Throughput: %.2f MB/s\n", float64(totalSize)/(1024*1024)/totalDuration.Seconds())
	fmt.Printf("Integrity Check: %d failures out of %d files\n", integrityFailures, successCount)
	
	if integrityFailures > 0 {
		fmt.Printf("WARNING: %d files failed integrity check!\n", integrityFailures)
	} else {
		fmt.Printf("✓ All files passed integrity verification\n")
	}
	
	fmt.Println("\nDetailed Results:")
	fmt.Println("-"*80)
	fmt.Printf("%-25s %10s %12s %10s %10s %s\n", "Filename", "Size(MB)", "Time", "MB/s", "Integrity", "Hash")
	fmt.Println("-"*80)
	
	for i, result := range bs.results {
		if result.Success {
			integrity := "✓"
			if !result.Integrity {
				integrity = "✗"
			}
			
			filename := fmt.Sprintf("File %d", i+1)
			if len(bs.results) <= 10 {
				// Show actual filenames for smaller result sets
				testCases := []string{
					"small_16bit_mono.wav", "small_16bit_stereo.wav", "medium_24bit_stereo.wav",
					"large_24bit_stereo.wav", "xlarge_24bit_stereo.wav", "sine_test.wav",
					"noise_test.wav", "silence_test.wav",
				}
				if i < len(testCases) {
					filename = testCases[i]
				}
			}
			
			fmt.Printf("%-25s %10.2f %12v %10.2f %10s %s\n",
				filename,
				float64(result.FileSize)/(1024*1024),
				result.Duration.Truncate(time.Millisecond),
				result.Throughput,
				integrity,
				result.Hash[:16]+"...",
			)
		} else {
			fmt.Printf("%-25s %10s %12s %10s %10s ERROR: %s\n",
				fmt.Sprintf("File %d", i+1), "N/A", "N/A", "N/A", "N/A", result.Error)
		}
	}
}

// CreateTestSuite creates a comprehensive test suite with various file types
func CreateTestSuite(outputDir string) error {
	generator := NewWAVGenerator(outputDir)
	
	// Comprehensive test cases covering edge cases and quality scenarios
	testSuites := map[string][]WAVSpec{
		"integrity": {
			{"integrity_small.wav", 10, 44100, 16, 2, "predictable"},
			{"integrity_medium.wav", 60, 48000, 24, 2, "predictable"},
			{"integrity_large.wav", 300, 96000, 24, 2, "predictable"},
		},
		"quality": {
			{"cd_quality.wav", 60, 44100, 16, 2, "sine"},
			{"studio_quality.wav", 60, 48000, 24, 2, "sine"},
			{"high_res.wav", 60, 96000, 24, 2, "sine"},
			{"ultra_high_res.wav", 60, 192000, 24, 2, "sine"},
		},
		"channels": {
			{"mono_test.wav", 30, 44100, 16, 1, "predictable"},
			{"stereo_test.wav", 30, 44100, 16, 2, "predictable"},
		},
		"patterns": {
			{"sine_440hz.wav", 30, 44100, 16, 2, "sine"},
			{"white_noise.wav", 30, 44100, 16, 2, "noise"},
			{"digital_silence.wav", 30, 44100, 16, 2, "silence"},
			{"predictable_pattern.wav", 30, 44100, 16, 2, "predictable"},
		},
		"stress": {
			{"stress_small_1.wav", 5, 22050, 16, 1, "predictable"},
			{"stress_small_2.wav", 5, 22050, 16, 1, "predictable"},
			{"stress_small_3.wav", 5, 22050, 16, 1, "predictable"},
			{"stress_small_4.wav", 5, 22050, 16, 1, "predictable"},
			{"stress_small_5.wav", 5, 22050, 16, 1, "predictable"},
		},
	}
	
	for suiteName, specs := range testSuites {
		suiteDir := filepath.Join(outputDir, suiteName)
		if err := os.MkdirAll(suiteDir, 0755); err != nil {
			return fmt.Errorf("failed to create suite directory %s: %w", suiteDir, err)
		}
		
		suiteGenerator := NewWAVGenerator(suiteDir)
		
		fmt.Printf("\nGenerating %s test suite...\n", suiteName)
		
		for _, spec := range specs {
			filePath, hash, err := suiteGenerator.GenerateWAV(spec)
			if err != nil {
				fmt.Printf("  ERROR generating %s: %v\n", spec.Filename, err)
				continue
			}
			
			fileInfo, _ := os.Stat(filePath)
			fmt.Printf("  ✓ %s (%.2f MB, hash: %s)\n", 
				spec.Filename, 
				float64(fileInfo.Size())/(1024*1024), 
				hash[:16]+"...")
		}
	}
	
	return nil
}

func main() {
	var (
		outputDir = flag.String("output", "./test-wavs", "Output directory for generated WAV files")
		benchmark = flag.Bool("benchmark", false, "Run benchmark suite")
		testSuite = flag.Bool("testsuite", false, "Generate comprehensive test suite")
	)
	flag.Parse()
	
	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	
	if *testSuite {
		fmt.Println("Generating comprehensive test suite...")
		if err := CreateTestSuite(*outputDir); err != nil {
			log.Fatalf("Failed to create test suite: %v", err)
		}
		fmt.Printf("\nTest suite generated in: %s\n", *outputDir)
	}
	
	if *benchmark {
		suite := NewBenchmarkSuite(*outputDir)
		if err := suite.RunBenchmarks(); err != nil {
			log.Fatalf("Benchmark failed: %v", err)
		}
	}
	
	if !*benchmark && !*testSuite {
		fmt.Println("WAV Generator for Audio Upload Integrity Testing")
		fmt.Println("Usage:")
		fmt.Println("  -benchmark     Run benchmark suite")
		fmt.Println("  -testsuite     Generate comprehensive test files")
		fmt.Println("  -output        Output directory (default: ./test-wavs)")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run wav-generator.go -benchmark")
		fmt.Println("  go run wav-generator.go -testsuite -output ./custom-test-files")
		fmt.Println("  go run wav-generator.go -benchmark -testsuite")
	}
}