package services

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AudioMetadata struct {
	// Basic file info
	Filename   string    `json:"filename"`
	FileSize   int64     `json:"file_size"`
	UploadTime time.Time `json:"upload_time"`

	// Audio format info
	Format        string  `json:"format"`
	Codec         string  `json:"codec"`
	Duration      float64 `json:"duration_seconds"`
	DurationText  string  `json:"duration_formatted"`
	Bitrate       int     `json:"bitrate_kbps"`
	SampleRate    int     `json:"sample_rate_hz"`
	Channels      int     `json:"channels"`
	BitsPerSample int     `json:"bits_per_sample"`

	// Quality metrics
	IsLossless bool   `json:"is_lossless"`
	Quality    string `json:"quality_assessment"`

	// Basic metadata tags (from file)
	Title   string `json:"title,omitempty"`
	Artist  string `json:"artist,omitempty"`
	Album   string `json:"album,omitempty"`
	Date    string `json:"date,omitempty"`
	Genre   string `json:"genre,omitempty"`
	Comment string `json:"comment,omitempty"`

	// Sermon-specific metadata (manually entered)
	SermonInfo SermonMetadata `json:"sermon_info,omitempty"`

	// Technical details
	AudioProfile string `json:"audio_profile,omitempty"`
	StreamCount  int    `json:"stream_count"`

	// File integrity
	IsValid  bool     `json:"is_valid"`
	Warnings []string `json:"warnings,omitempty"`

	// Processing metrics
	ProcessingDuration time.Duration `json:"processing_duration,omitempty"`
}

// SermonMetadata contains sermon-specific information
type SermonMetadata struct {
	// Core sermon information
	SpeakerName  string    `json:"speaker_name,omitempty"`
	SermonTitle  string    `json:"sermon_title,omitempty"`
	SermonTheme  string    `json:"sermon_theme,omitempty"`
	SermonDate   time.Time `json:"sermon_date,omitempty"`
	SermonSeries string    `json:"sermon_series,omitempty"`

	// Biblical references
	BibleVerses []BibleVerse `json:"bible_verses,omitempty"`
	MainPassage string       `json:"main_passage,omitempty"`

	// Additional context
	ChurchEvent string `json:"church_event,omitempty"` // e.g., "Sunday Service", "Bible Study"
	ServiceType string `json:"service_type,omitempty"` // e.g., "Morning Service", "Evening Service"
	Audience    string `json:"audience,omitempty"`     // e.g., "General", "Youth", "Children"
	Language    string `json:"language,omitempty"`     // e.g., "English", "Spanish"

	// Quality/Content notes
	Summary   string   `json:"summary,omitempty"`
	KeyPoints []string `json:"key_points,omitempty"`
	Tags      []string `json:"tags,omitempty"` // Custom tags for categorization

	// Administrative
	ApprovedBy          string `json:"approved_by,omitempty"` // Who approved this for sharing
	IsPublic            bool   `json:"is_public"`             // Whether it can be shared publicly
	TranscriptAvailable bool   `json:"transcript_available"`  // Whether transcript exists

	// Timestamps within the recording
	IntroEnd   float64     `json:"intro_end_seconds,omitempty"`  // When sermon actually starts
	SermonEnd  float64     `json:"sermon_end_seconds,omitempty"` // When sermon ends (before announcements)
	KeyMoments []Timestamp `json:"key_moments,omitempty"`        // Important moments in the sermon
}

// BibleVerse represents a biblical reference
type BibleVerse struct {
	Book      string `json:"book"`                 // e.g., "Matthew"
	Chapter   int    `json:"chapter"`              // e.g., 5
	VerseFrom int    `json:"verse_from,omitempty"` // Starting verse
	VerseTo   int    `json:"verse_to,omitempty"`   // Ending verse (for ranges)
	Text      string `json:"text,omitempty"`       // Actual verse text if available
	Version   string `json:"version,omitempty"`    // Bible version (NIV, ESV, etc.)
}

// Timestamp represents a significant moment in the recording
type Timestamp struct {
	Time        float64 `json:"time_seconds"`
	Description string  `json:"description"`
	Type        string  `json:"type"` // e.g., "key_point", "prayer", "invitation", "scripture_reading"
}

type FFProbeOutput struct {
	Streams []struct {
		CodecName     string `json:"codec_name"`
		CodecLongName string `json:"codec_long_name"`
		Profile       string `json:"profile,omitempty"`
		CodecType     string `json:"codec_type"`
		SampleRate    string `json:"sample_rate,omitempty"`
		Channels      int    `json:"channels,omitempty"`
		ChannelLayout string `json:"channel_layout,omitempty"`
		BitsPerSample int    `json:"bits_per_sample,omitempty"`
		Duration      string `json:"duration,omitempty"`
		BitRate       string `json:"bit_rate,omitempty"`
	} `json:"streams"`
	Format struct {
		Filename       string            `json:"filename"`
		FormatName     string            `json:"format_name"`
		FormatLongName string            `json:"format_long_name"`
		Duration       string            `json:"duration,omitempty"`
		Size           string            `json:"size"`
		BitRate        string            `json:"bit_rate,omitempty"`
		Tags           map[string]string `json:"tags,omitempty"`
	} `json:"format"`
}

type MetadataService struct {
	tempDir string
}

func NewMetadataService(tempDir string) *MetadataService {
	return &MetadataService{
		tempDir: tempDir,
	}
}

// ExtractMetadataFromFile analyzes a local file and extracts comprehensive metadata
func (m *MetadataService) ExtractMetadataFromFile(filePath string) (*AudioMetadata, error) {
	// Get basic file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	metadata := &AudioMetadata{
		Filename:   filepath.Base(filePath),
		FileSize:   fileInfo.Size(),
		UploadTime: time.Now(),
		IsValid:    false,
	}

	// Use ffprobe to extract detailed audio metadata
	if err := m.extractFFProbeMetadata(filePath, metadata); err != nil {
		metadata.Warnings = append(metadata.Warnings, fmt.Sprintf("FFProbe analysis failed: %v", err))
	}

	// Assess audio quality
	m.assessAudioQuality(metadata)

	// Validate file integrity
	m.validateFileIntegrity(filePath, metadata)

	return metadata, nil
}

// ExtractMetadataFromMinIO downloads file temporarily and extracts metadata
func (m *MetadataService) ExtractMetadataFromMinIO(minioService *MinIOService, filename string) (*AudioMetadata, error) {
	// Create temp file path
	tempFilePath := filepath.Join(m.tempDir, filename)

	// Ensure temp directory exists
	if err := os.MkdirAll(filepath.Dir(tempFilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}

	// Download file from MinIO to temp location
	if err := minioService.DownloadFile(filename, tempFilePath); err != nil {
		return nil, fmt.Errorf("failed to download file for analysis: %v", err)
	}

	// Extract metadata
	metadata, err := m.ExtractMetadataFromFile(tempFilePath)

	// Cleanup temp file
	os.Remove(tempFilePath)

	return metadata, err
}

func (m *MetadataService) extractFFProbeMetadata(filePath string, metadata *AudioMetadata) error {
	// Run ffprobe to get JSON metadata
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffprobe execution failed: %v", err)
	}

	var probe FFProbeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return fmt.Errorf("failed to parse ffprobe output: %v", err)
	}

	// Extract format information
	metadata.Format = probe.Format.FormatName
	if duration, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
		metadata.Duration = duration
		metadata.DurationText = m.formatDuration(duration)
	}
	if bitrate, err := strconv.Atoi(probe.Format.BitRate); err == nil {
		metadata.Bitrate = bitrate / 1000 // Convert to kbps
	}

	// Extract metadata tags
	if probe.Format.Tags != nil {
		metadata.Title = probe.Format.Tags["title"]
		metadata.Artist = probe.Format.Tags["artist"]
		metadata.Album = probe.Format.Tags["album"]
		metadata.Date = probe.Format.Tags["date"]
		metadata.Genre = probe.Format.Tags["genre"]
		metadata.Comment = probe.Format.Tags["comment"]
	}

	// Extract audio stream information
	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" {
			metadata.Codec = stream.CodecName
			metadata.AudioProfile = stream.Profile
			metadata.Channels = stream.Channels
			metadata.BitsPerSample = stream.BitsPerSample

			if sampleRate, err := strconv.Atoi(stream.SampleRate); err == nil {
				metadata.SampleRate = sampleRate
			}

			// Determine if lossless
			metadata.IsLossless = m.isLosslessCodec(stream.CodecName)

			break // Use first audio stream
		}
	}

	metadata.StreamCount = len(probe.Streams)
	metadata.IsValid = true

	return nil
}

func (m *MetadataService) isLosslessCodec(codec string) bool {
	losslessCodecs := []string{"pcm_s16le", "pcm_s24le", "pcm_s32le", "flac", "alac", "ape", "wavpack"}
	for _, lossless := range losslessCodecs {
		if strings.Contains(strings.ToLower(codec), lossless) {
			return true
		}
	}
	return false
}

func (m *MetadataService) assessAudioQuality(metadata *AudioMetadata) {
	// Quality assessment based on technical parameters
	score := 0

	// Sample rate scoring
	switch {
	case metadata.SampleRate >= 96000:
		score += 4 // Hi-res
	case metadata.SampleRate >= 48000:
		score += 3 // Professional
	case metadata.SampleRate >= 44100:
		score += 2 // CD quality
	default:
		score += 1 // Below CD quality
	}

	// Bit depth scoring
	switch {
	case metadata.BitsPerSample >= 24:
		score += 3 // High resolution
	case metadata.BitsPerSample >= 16:
		score += 2 // CD quality
	default:
		score += 1 // Lower quality
	}

	// Lossless bonus
	if metadata.IsLossless {
		score += 2
	}

	// Bitrate consideration (for lossy formats)
	if !metadata.IsLossless && metadata.Bitrate >= 320 {
		score += 1
	}

	// Quality labels
	switch {
	case score >= 8:
		metadata.Quality = "Excellent (Hi-Res)"
	case score >= 6:
		metadata.Quality = "Very Good (Professional)"
	case score >= 4:
		metadata.Quality = "Good (CD Quality)"
	default:
		metadata.Quality = "Fair (Compressed)"
	}
}

func (m *MetadataService) validateFileIntegrity(filePath string, metadata *AudioMetadata) {
	// Use ffmpeg to validate file integrity
	cmd := exec.Command("ffmpeg",
		"-v", "error",
		"-i", filePath,
		"-f", "null", "-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		metadata.Warnings = append(metadata.Warnings, "File integrity check failed")
		metadata.IsValid = false
		return
	}

	// Check for any error messages in output
	if len(output) > 0 {
		metadata.Warnings = append(metadata.Warnings, fmt.Sprintf("Audio stream issues detected: %s", string(output)))
	}
}

func (m *MetadataService) formatDuration(seconds float64) string {
	duration := time.Duration(seconds * float64(time.Second))
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// CalculateStreamingHash calculates SHA256 hash of a stream without loading entire content into memory
func (m *MetadataService) CalculateStreamingHash(reader io.Reader) (string, error) {
	hasher := sha256.New()
	
	// Use 32KB buffer for streaming hash calculation
	buffer := make([]byte, 32768)
	
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			hasher.Write(buffer[:n])
		}
		
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read stream for hashing: %w", err)
		}
	}
	
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
