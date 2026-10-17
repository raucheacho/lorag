package parsers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
)

// WhisperModel represents available whisper models
type WhisperModel string

const (
	WhisperTiny  WhisperModel = "ggml-tiny.bin"
	WhisperBase  WhisperModel = "ggml-base.bin"
	WhisperSmall WhisperModel = "ggml-small.bin"
)

// ModelStatus represents the status of a model
type ModelStatus int

const (
	ModelStatusNotDownloaded ModelStatus = iota
	ModelStatusDownloading
	ModelStatusReady
	ModelStatusError
)

// DownloadProgress callback for download progress
type DownloadProgress func(model WhisperModel, downloaded, total int64)

// WhisperModelManager manages whisper.cpp binary and models
type WhisperModelManager struct {
	dataDir          string
	progressCallback DownloadProgress
}

// NewWhisperModelManager creates a new manager
func NewWhisperModelManager(dataDir string) *WhisperModelManager {
	return &WhisperModelManager{dataDir: dataDir}
}

// Progress of a model download, shared by every manager of the process.
type Progress struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"` // 0 while unknown
	Error      string `json:"error,omitempty"`
}

type downloadState struct {
	running           atomic.Bool
	downloaded, total atomic.Int64
	mu                sync.Mutex
	err               string
}

// states holds the download state of each model path.
var states sync.Map // model path → *downloadState

func stateOf(path string) *downloadState {
	v, _ := states.LoadOrStore(path, &downloadState{})
	return v.(*downloadState)
}

func (m *WhisperModelManager) modelPath(model WhisperModel) string {
	return filepath.Join(m.dataDir, "models", string(model))
}

// SetProgressCallback sets a callback for download progress
func (m *WhisperModelManager) SetProgressCallback(cb DownloadProgress) {
	m.progressCallback = cb
}

// downloads serializes model downloads across managers: parsers create
// their own manager, and audio and video files are indexed in parallel.
var downloads sync.Map // model path → *sync.Mutex

// GetStatus returns the current status of a model
func (m *WhisperModelManager) GetStatus(model WhisperModel) ModelStatus {
	path := m.modelPath(model)
	st := stateOf(path)
	if st.running.Load() {
		return ModelStatusDownloading
	}
	if _, err := os.Stat(path); err == nil {
		return ModelStatusReady
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != "" {
		return ModelStatusError
	}
	return ModelStatusNotDownloaded
}

// Progress returns how far the current or last download of a model went.
func (m *WhisperModelManager) Progress(model WhisperModel) Progress {
	st := stateOf(m.modelPath(model))
	st.mu.Lock()
	defer st.mu.Unlock()
	return Progress{Downloaded: st.downloaded.Load(), Total: st.total.Load(), Error: st.err}
}

// Download fetches a model in the background, unless it is present. A
// download already running is not started twice: GetModelPath waits for it.
func (m *WhisperModelManager) Download(model WhisperModel) {
	if m.GetStatus(model) == ModelStatusReady {
		return
	}
	// Downloading from now on, so a status read right after this call
	// does not report the model as missing.
	st := stateOf(m.modelPath(model))
	st.running.Store(true)
	go func() {
		if _, err := m.GetModelPath(model); err != nil {
			st.mu.Lock()
			st.err = err.Error()
			st.mu.Unlock()
		}
		st.running.Store(false)
	}()
}

// GetModelPath returns the path to a model, downloading it if necessary
func (m *WhisperModelManager) GetModelPath(model WhisperModel) (string, error) {
	modelPath := m.modelPath(model)

	// One download per model: later callers wait, then find the file.
	lock, _ := downloads.LoadOrStore(modelPath, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	// Check if model exists
	if _, err := os.Stat(modelPath); err == nil {
		return modelPath, nil
	}

	// Create models directory
	if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create models directory: %w", err)
	}

	// Download model
	if err := m.downloadModel(model, modelPath); err != nil {
		return "", err
	}
	return modelPath, nil
}

// downloadModel downloads a whisper model from HuggingFace
func (m *WhisperModelManager) downloadModel(model WhisperModel, destPath string) (err error) {
	st := stateOf(destPath)
	st.mu.Lock()
	st.err = ""
	st.mu.Unlock()
	st.downloaded.Store(0)
	st.total.Store(0)
	st.running.Store(true)
	defer func() {
		if err != nil {
			st.mu.Lock()
			st.err = err.Error()
			st.mu.Unlock()
		}
		st.running.Store(false)
	}()

	url := fmt.Sprintf("https://huggingface.co/ggerganov/whisper.cpp/resolve/main/%s", model)

	fmt.Printf("Downloading whisper model %s...\n", model)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: status %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	if totalSize > 0 {
		st.total.Store(totalSize)
	}

	// Temporary file next to the destination, renamed once complete.
	out, err := os.CreateTemp(filepath.Dir(destPath), filepath.Base(destPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFile := out.Name()

	// Copy with progress tracking
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				st.downloaded.Store(written)
				if m.progressCallback != nil && totalSize > 0 {
					m.progressCallback(model, written, totalSize)
				}
			}
			if ew != nil {
				out.Close()
				os.Remove(tmpFile)
				return fmt.Errorf("failed to write model: %w", ew)
			}
		}
		if er != nil {
			if er != io.EOF {
				out.Close()
				os.Remove(tmpFile)
				return fmt.Errorf("failed to download model: %w", er)
			}
			break
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to write model: %w", err)
	}

	// Rename to final destination
	if err := os.Rename(tmpFile, destPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to save model: %w", err)
	}

	fmt.Printf("Downloaded %s (%.1f MB)\n", model, float64(written)/(1024*1024))
	return nil
}

// GetWhisperCppPath returns the path to whisper.cpp binary
func (m *WhisperModelManager) GetWhisperCppPath() (string, error) {
	binaryName := "whisper-cli"
	if runtime.GOOS == "windows" {
		binaryName = "whisper-cli.exe"
	}

	binaryPath := filepath.Join(m.dataDir, "bin", binaryName)

	// Check if binary exists in our data dir
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	// Try system whisper
	if path, err := exec.LookPath("whisper-cli"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("whisper-cpp"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("main"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("WHISPER_CPP_NOT_FOUND: whisper.cpp n'est pas installé.\n\n" +
		"Pour utiliser la transcription locale, vous devez installer whisper.cpp:\n" +
		"  1. git clone https://github.com/ggerganov/whisper.cpp.git\n" +
		"  2. cd whisper.cpp\n" +
		"  3. make\n" +
		"  4. sudo cp main /usr/local/bin/whisper-cli")
}

// GetDefaultModel returns the default model (tiny)
func (m *WhisperModelManager) GetDefaultModel() WhisperModel {
	return WhisperTiny
}

// ListAvailableModels returns list of available model sizes
func (m *WhisperModelManager) ListAvailableModels() []WhisperModel {
	return []WhisperModel{WhisperTiny, WhisperBase, WhisperSmall}
}

// GetModelSize returns the size of a model in MB
func (m *WhisperModelManager) GetModelSize(model WhisperModel) float64 {
	switch model {
	case WhisperTiny:
		return 39.0
	case WhisperBase:
		return 74.0
	case WhisperSmall:
		return 244.0
	default:
		return 0
	}
}

// IsModelDownloaded checks if a model is already downloaded
func (m *WhisperModelManager) IsModelDownloaded(model WhisperModel) bool {
	return m.GetStatus(model) == ModelStatusReady
}

// GetStatusText returns human-readable status
func (m *WhisperModelManager) GetStatusText(model WhisperModel) string {
	switch m.GetStatus(model) {
	case ModelStatusNotDownloaded:
		return "not_downloaded"
	case ModelStatusDownloading:
		return "downloading"
	case ModelStatusReady:
		return "ready"
	case ModelStatusError:
		return "error"
	default:
		return "unknown"
	}
}
