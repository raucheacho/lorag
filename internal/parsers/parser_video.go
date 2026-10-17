package parsers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// VideoParser extracts audio from video and transcribes it using whisper.cpp
type VideoParser struct {
	audioParser *AudioParser
	timeout     time.Duration
}

// NewVideoParser creates a new VideoParser with default settings
func NewVideoParser() *VideoParser {
	return NewVideoParserWithConfig("", WhisperTiny)
}

// NewVideoParserWithConfig creates a new VideoParser with custom config
func NewVideoParserWithConfig(dataDir string, model WhisperModel) *VideoParser {
	return &VideoParser{
		audioParser: NewAudioParserWithConfig(dataDir, model),
		timeout:     300 * time.Second,
	}
}

// SetModel changes the model to use
func (p *VideoParser) SetModel(model WhisperModel) {
	p.audioParser.SetModel(model)
}

// checkFFmpeg checks if ffmpeg is installed
func (p *VideoParser) checkFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("FFMPEG_NOT_INSTALLED: ffmpeg n'est pas installé.\n\n" +
			"Pour indexer des vidéos, vous devez installer ffmpeg:\n" +
			"  • macOS:   brew install ffmpeg\n" +
			"  • Ubuntu:  apt install ffmpeg\n" +
			"  • Windows: choco install ffmpeg\n" +
			"  • Autres:  https://ffmpeg.org/download.html")
	}
	return nil
}

// ParseFile extracts audio from video and transcribes it
func (p *VideoParser) ParseFile(path string) (string, error) {
	// Check ffmpeg first
	if err := p.checkFFmpeg(); err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat video: %w", err)
	}

	baseDesc := p.formatVideoMetadata(path, info)

	// Create temp directory for extraction
	tempDir, err := os.MkdirTemp("", "lorag_video_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract audio to temp file
	audioPath := filepath.Join(tempDir, "audio.wav")
	if err := p.extractAudio(path, audioPath); err != nil {
		return "", fmt.Errorf("FFMPEG_EXTRACTION_FAILED: Échec de l'extraction audio: %w", err)
	}

	// Check if audio file was created and has content
	audioInfo, err := os.Stat(audioPath)
	if err != nil || audioInfo.Size() == 0 {
		return baseDesc + "\n=== Audio ===\n[Aucune piste audio trouvée]", nil
	}

	// Transcribe with whisper
	transcription, err := p.audioParser.ParseFile(audioPath)
	if err != nil {
		return "", err // Propagate whisper errors
	}

	// Extract just the transcription part (remove metadata)
	parts := strings.SplitN(transcription, "=== Transcription ===\n", 2)
	if len(parts) == 2 {
		return baseDesc + "\n=== Transcription ===\n" + parts[1], nil
	}

	return baseDesc + "\n" + transcription, nil
}

// extractAudio extracts audio from video using ffmpeg
func (p *VideoParser) extractAudio(videoPath, audioPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vn",                  // No video
		"-acodec", "pcm_s16le", // PCM 16-bit little-endian
		"-ar", "16000", // 16kHz sample rate (whisper optimal)
		"-ac", "1", // Mono
		"-y", // Overwrite output
		audioPath,
	)

	// Capture stderr for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		// ffmpeg returns error code even for warnings, check if file was created
		if _, statErr := os.Stat(audioPath); statErr == nil {
			return nil // File exists, probably succeeded
		}
		return fmt.Errorf("ffmpeg failed: %v, output: %s", err, string(output))
	}
	return nil
}

// formatVideoMetadata creates base description with video metadata
func (p *VideoParser) formatVideoMetadata(path string, info os.FileInfo) string {
	ext := strings.ToLower(filepath.Ext(path))
	var format string
	switch ext {
	case ".mp4":
		format = "MP4"
	case ".avi":
		format = "AVI"
	case ".mkv":
		format = "MKV"
	case ".mov":
		format = "QuickTime"
	case ".webm":
		format = "WebM"
	case ".flv":
		format = "FLV"
	case ".wmv":
		format = "WMV"
	default:
		format = "Video"
	}

	return fmt.Sprintf("=== %s Video ===\nFile: %s\nSize: %d bytes", format, filepath.Base(path), info.Size())
}

// Supports returns supported video extensions
func (p *VideoParser) Supports(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp4", ".avi", ".mkv", ".mov", ".webm", ".flv", ".wmv", ".m4v":
		return true
	}
	return false
}

// GetModelManager returns the model manager for configuration
func (p *VideoParser) GetModelManager() *WhisperModelManager {
	return p.audioParser.GetModelManager()
}

// maxKeyframes bounds the frames read on screen per video.
const maxKeyframes = 120

// Segments returns the timed transcription of the audio track, merged in
// time order with the text read on screen.
func (p *VideoParser) Segments(path string) ([]Segment, error) {
	if err := p.checkFFmpeg(); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "lorag_video_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	var segs []Segment
	audioPath := filepath.Join(dir, "audio.wav")
	speechErr := p.extractAudio(path, audioPath)
	if speechErr == nil {
		if info, err := os.Stat(audioPath); err == nil && info.Size() > 44 {
			var s []Segment
			s, speechErr = p.audioParser.Segments(audioPath)
			segs = append(segs, s...)
		}
	}
	segs = append(segs, keyframeText(path, dir)...)
	if len(segs) == 0 {
		if speechErr != nil {
			return nil, speechErr
		}
		return nil, fmt.Errorf("NO_VIDEO_TEXT: no speech and no text on screen")
	}
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].StartMs < segs[j].StartMs })
	return segs, nil
}

var ptsTime = regexp.MustCompile(`pts_time:([0-9.]+)`)

// keyframeText reads the text on screen: a frame at each scene change and at
// least one every interval (slides barely change the picture, so scene
// detection alone misses them), read with tesseract. Frames without readable
// text, or with the same text as before, are dropped.
func keyframeText(path, dir string) []Segment {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return nil
	}
	interval := 5.0
	if d := duration(path); d/maxKeyframes > interval {
		interval = d / maxKeyframes
	}
	cmd := exec.Command("ffmpeg", "-hide_banner", "-i", path,
		"-vf", fmt.Sprintf("select='eq(n\\,0)+gt(scene\\,0.2)+gte(t-prev_selected_t\\,%.1f)',showinfo,scale='min(1600,iw)':-2", interval),
		"-fps_mode", "vfr", "-frames:v", strconv.Itoa(maxKeyframes),
		filepath.Join(dir, "frame_%03d.png"), "-y")
	out, _ := cmd.CombinedOutput()
	times := ptsTime.FindAllStringSubmatch(string(out), -1)

	var segs []Segment
	seen := map[string]bool{}
	for i, m := range times {
		frame := filepath.Join(dir, fmt.Sprintf("frame_%03d.png", i+1))
		text := ocr(frame)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		sec, _ := strconv.ParseFloat(m[1], 64)
		ms := int64(sec * 1000)
		segs = append(segs, Segment{Text: "On screen: " + text, StartMs: ms, EndMs: ms + 1, Offset: -1, Section: "screen"})
	}
	return segs
}

// duration returns the length of a media file in seconds, 0 when unknown.
func duration(path string) float64 {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
	if err != nil {
		return 0
	}
	d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return d
}
