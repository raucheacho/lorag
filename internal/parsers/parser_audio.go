package parsers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AudioParser transcribes audio using whisper.cpp
type AudioParser struct {
	modelManager *WhisperModelManager
	model        WhisperModel
	timeout      time.Duration
}

// NewAudioParser creates a new AudioParser with default settings
func NewAudioParser() *AudioParser {
	// Default data directory in user's home
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".lorag")

	return NewAudioParserWithConfig(dataDir, WhisperTiny)
}

// NewAudioParserWithConfig creates a new AudioParser with custom config
func NewAudioParserWithConfig(dataDir string, model WhisperModel) *AudioParser {
	return &AudioParser{
		modelManager: NewWhisperModelManager(dataDir),
		model:        model,
		timeout:      300 * time.Second,
	}
}

// SetModel changes the model to use
func (p *AudioParser) SetModel(model WhisperModel) {
	p.model = model
}

// ParseFile transcribes an audio file
func (p *AudioParser) ParseFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat audio: %w", err)
	}

	baseDesc := p.formatAudioMetadata(path, info)

	// Get whisper.cpp binary
	whisperPath, err := p.modelManager.GetWhisperCppPath()
	if err != nil {
		return "", err
	}

	// Get model (auto-download if needed)
	modelPath, err := p.modelManager.GetModelPath(p.model)
	if err != nil {
		return "", fmt.Errorf("WHISPER_MODEL_ERROR: %w", err)
	}

	// Transcribe
	text, err := p.transcribe(whisperPath, modelPath, path)
	if err != nil {
		return "", err
	}

	if text == "" {
		return baseDesc + "\n=== Transcription ===\n[Aucune parole détectée]", nil
	}

	return baseDesc + "\n=== Transcription ===\n" + text, nil
}

// transcribe runs whisper.cpp on the audio file
func (p *AudioParser) transcribe(whisperPath, modelPath, audioPath string) (string, error) {
	// Convert to WAV if needed (whisper.cpp works best with WAV)
	wavPath, err := p.ensureWav(audioPath)
	if err != nil {
		return "", err
	}
	if wavPath != audioPath {
		defer os.Remove(wavPath)
	}

	// Run whisper.cpp
	cmd := exec.Command(whisperPath,
		"-m", modelPath,
		"-f", wavPath,
		"-l", "auto",
		"--no-timestamps",
	)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("WHISPER_TRANSCRIPTION_FAILED: Échec de la transcription: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// ensureWav converts audio to WAV format if needed
func (p *AudioParser) ensureWav(audioPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(audioPath))
	if ext == ".wav" {
		return audioPath, nil
	}

	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		// Try to use the file as-is
		return audioPath, nil
	}

	// Convert to WAV
	tempDir := os.TempDir()
	wavPath := filepath.Join(tempDir, fmt.Sprintf("lorag_%d.wav", time.Now().UnixNano()))

	cmd := exec.Command("ffmpeg",
		"-i", audioPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-y",
		wavPath,
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return audioPath, nil // Fallback to original
	}

	return wavPath, nil
}

// formatAudioMetadata creates base description with audio metadata
func (p *AudioParser) formatAudioMetadata(path string, info os.FileInfo) string {
	ext := strings.ToLower(filepath.Ext(path))
	var format string
	switch ext {
	case ".wav":
		format = "WAV"
	case ".mp3":
		format = "MP3"
	case ".m4a":
		format = "M4A"
	case ".ogg", ".oga":
		format = "OGG"
	case ".flac":
		format = "FLAC"
	case ".wma":
		format = "WMA"
	case ".aac":
		format = "AAC"
	default:
		format = "Audio"
	}

	return fmt.Sprintf("=== %s Audio ===\nFile: %s\nSize: %d bytes", format, filepath.Base(path), info.Size())
}

// Supports returns supported audio extensions
func (p *AudioParser) Supports(ext string) bool {
	switch strings.ToLower(ext) {
	case ".wav", ".mp3", ".m4a", ".aac", ".ogg", ".oga", ".flac", ".wma":
		return true
	}
	return false
}

// GetModelManager returns the model manager for configuration
func (p *AudioParser) GetModelManager() *WhisperModelManager {
	return p.modelManager
}

// Segments transcribes the file into timed segments (whisper.cpp JSON
// output, one segment per stretch of speech).
func (p *AudioParser) Segments(path string) ([]Segment, error) {
	whisperPath, err := p.modelManager.GetWhisperCppPath()
	if err != nil {
		return nil, err
	}
	modelPath, err := p.modelManager.GetModelPath(p.model)
	if err != nil {
		return nil, fmt.Errorf("WHISPER_MODEL_ERROR: %w", err)
	}
	wavPath, err := p.ensureWav(path)
	if err != nil {
		return nil, err
	}
	if wavPath != path {
		defer os.Remove(wavPath)
	}
	dir, err := os.MkdirTemp("", "lorag_whisper_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "out")
	cmd := exec.Command(whisperPath, "-m", modelPath, "-f", wavPath, "-l", "auto", "-oj", "-of", prefix, "-np")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("WHISPER_TRANSCRIPTION_FAILED: %w: %s", err, lastLine(out))
	}
	data, err := os.ReadFile(prefix + ".json")
	if err != nil {
		return nil, fmt.Errorf("WHISPER_TRANSCRIPTION_FAILED: no JSON output: %w", err)
	}
	return parseWhisperJSON(data)
}

// parseWhisperJSON reads whisper.cpp's --output-json format.
func parseWhisperJSON(data []byte) ([]Segment, error) {
	var out struct {
		Transcription []struct {
			Offsets struct {
				From int64 `json:"from"`
				To   int64 `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("WHISPER_TRANSCRIPTION_FAILED: bad JSON: %w", err)
	}
	segs := make([]Segment, 0, len(out.Transcription))
	for _, t := range out.Transcription {
		text := strings.TrimSpace(t.Text)
		if text == "" || text == "[BLANK_AUDIO]" {
			continue
		}
		end := t.Offsets.To
		if end <= t.Offsets.From {
			end = t.Offsets.From + 1
		}
		segs = append(segs, Segment{Text: text, StartMs: t.Offsets.From, EndMs: end, Offset: -1})
	}
	if len(segs) == 0 {
		return nil, fmt.Errorf("NO_SPEECH: no speech found in the audio")
	}
	return segs, nil
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return lines[len(lines)-1]
}
