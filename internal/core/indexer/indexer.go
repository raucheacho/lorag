package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/core/vector"
	"github.com/lorag/lorag/internal/embeddings"
	"github.com/lorag/lorag/internal/parsers"
)

// Indexer handles document indexing
type Indexer struct {
	store         *store.BoltStore
	settingsStore *store.SettingsStore
	vectorIdx     vector.Index
	embedder      embeddings.Provider
	parserReg     *parsers.Registry
	chunker       *Chunker
	maxChunks     int
	queue         *Queue // Async queue for parallel indexing
}

// Config holds indexer configuration
type Config struct {
	MaxChunks    int
	ChunkSize    int
	ChunkOverlap int
	Workers      int // Number of concurrent workers
	MaxQueue     int // Max queue size
}

// NewIndexer creates a new indexer
func NewIndexer(
	st *store.BoltStore,
	settings *store.SettingsStore,
	vectorIdx vector.Index,
	embedder embeddings.Provider,
	cfg Config,
) *Indexer {
	// Load parser config from settings
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".lorag")
	
	parserCfg := parsers.ParserConfig{
		OcrEnabled:           false,
		OcrModel:             "llava:7b",
		OllamaURL:            "http://localhost:11434",
		TranscriptionEnabled: false,
		WhisperModel:         parsers.WhisperTiny,
		DataDir:              dataDir,
	}
	
	if settings != nil {
		if appSettings, err := settings.GetAppSettings(); err == nil {
			parserCfg.OcrEnabled = appSettings.OcrEnabled
			parserCfg.OcrModel = appSettings.OcrModel
			parserCfg.OllamaURL = appSettings.EmbeddingURL
			parserCfg.TranscriptionEnabled = appSettings.TranscriptionEnabled
			// Parse whisper model from string
			switch appSettings.WhisperModel {
			case "base":
				parserCfg.WhisperModel = parsers.WhisperBase
			case "small":
				parserCfg.WhisperModel = parsers.WhisperSmall
			default:
				parserCfg.WhisperModel = parsers.WhisperTiny
			}
		}
	}

	idx := &Indexer{
		store:         st,
		settingsStore: settings,
		vectorIdx:     vectorIdx,
		embedder:      embedder,
		parserReg:     parsers.NewRegistryWithConfig(parserCfg),
		chunker:       NewChunker(cfg.ChunkSize, cfg.ChunkOverlap),
		maxChunks:     cfg.MaxChunks,
	}

	// Initialize queue if workers configured
	if cfg.Workers > 0 {
		idx.queue = NewQueue(idx, cfg.Workers, cfg.MaxQueue)
	}

	return idx
}

// SubmitTask submits a file for async indexing
func (i *Indexer) SubmitTask(ctx context.Context, path string) (*Task, error) {
	if i.queue == nil {
		return nil, fmt.Errorf("async indexing not enabled")
	}
	return i.queue.Submit(ctx, path, TaskTypeFile)
}

// SubmitDirectoryTask submits a directory for async indexing
func (i *Indexer) SubmitDirectoryTask(ctx context.Context, path string) (*Task, error) {
	if i.queue == nil {
		return nil, fmt.Errorf("async indexing not enabled")
	}
	return i.queue.Submit(ctx, path, TaskTypeDirectory)
}

// GetQueueStats returns queue statistics
func (i *Indexer) GetQueueStats() map[string]interface{} {
	if i.queue == nil {
		return map[string]interface{}{"enabled": false}
	}
	stats := i.queue.GetStats()
	stats["enabled"] = true
	return stats
}

// GetQueue returns the queue (for accessing running/completed tasks)
func (i *Indexer) GetQueue() *Queue {
	return i.queue
}

// Stop gracefully stops the indexer and its queue
func (i *Indexer) Stop() {
	if i.queue != nil {
		i.queue.Stop()
	}
}

// IndexFile indexes a single file
func (i *Indexer) IndexFile(ctx context.Context, path string) (*store.Document, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", path)
	}
	
	// Check if already indexed and up to date
	existingDoc, err := i.store.GetDocumentByPath(path)
	if err == nil {
		// Check if file has changed
		checksum, err := i.calculateChecksum(path)
		if err != nil {
			return nil, err
		}
		
		if existingDoc.Checksum == checksum {
			// File hasn't changed, skip
			return existingDoc, nil
		}
		
		// File changed, delete old index
		if err := i.DeleteDocument(ctx, existingDoc.ID); err != nil {
			return nil, fmt.Errorf("failed to delete old index: %w", err)
		}
	}
	
	// Parse file
	content, err := i.parseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}
	
	// Create document
	ext := strings.ToLower(filepath.Ext(path))
	doc := store.NewDocument(path, filepath.Base(path), ext, info.Size())
	doc.Content = content
	
	checksum, err := i.calculateChecksum(path)
	if err != nil {
		return nil, err
	}
	doc.Checksum = checksum
	
	// Save document first
	if err := i.store.SaveDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to save document: %w", err)
	}
	
	// Chunk content
	chunks := i.chunker.Chunk(content)
	
	// Check chunk limit
	currentCount, _ := i.store.CountChunks()
	if currentCount+len(chunks) > i.maxChunks {
		// Delete document we just saved
		i.store.DeleteDocument(doc.ID)
		return nil, fmt.Errorf("max chunks limit reached (%d)", i.maxChunks)
	}
	
	// Process chunks
	for idx, chunkText := range chunks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Create chunk
		startByte := strings.Index(content, chunkText)
		if startByte < 0 {
			startByte = 0
		}
		endByte := startByte + len(chunkText)
		
		chunk := store.NewChunk(doc.ID, chunkText, idx, startByte, endByte)
		chunk.Tokens = i.chunker.CountTokens(chunkText)
		
		// Generate embedding
		embedding, err := i.embedder.Embed(chunkText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed chunk %d: %w", idx, err)
		}
		chunk.SetEmbedding(embedding)
		
		// Save chunk
		if err := i.store.SaveChunk(chunk); err != nil {
			return nil, fmt.Errorf("failed to save chunk: %w", err)
		}
		
		// Add to vector index
		if err := i.vectorIdx.Add(chunk.ID, embedding); err != nil {
			return nil, fmt.Errorf("failed to add to vector index: %w", err)
		}
	}
	
	// Mark document as indexed
	doc.MarkIndexed(len(chunks))
	if err := i.store.SaveDocument(doc); err != nil {
		return nil, err
	}
	
	return doc, nil
}

// IndexDirectory indexes all files in a directory
// If the indexer has a queue configured, files are processed in parallel
func (i *Indexer) IndexDirectory(ctx context.Context, dir string, recursive bool) (*IndexResult, error) {
	result := &IndexResult{
		StartTime: time.Now(),
	}

	// Collect all files to index
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}

		// Skip directories
		if info.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file type is supported
		if !i.isSupportedFile(path) {
			result.Skipped++
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return result, err
	}

	// If queue is available, use parallel processing
	if i.queue != nil && len(files) > 0 {
		return i.indexDirectoryParallel(ctx, files, result)
	}

	// Sequential processing (fallback)
	for _, path := range files {
		doc, err := i.IndexFile(ctx, path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
			result.Failed++
		} else {
			if doc.IndexedAt != nil && doc.IndexedAt.After(result.StartTime) {
				result.Indexed++
			} else {
				result.Skipped++
			}
			result.TotalChunks += doc.ChunkCount
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// indexDirectoryParallel processes files in parallel using the queue
func (i *Indexer) indexDirectoryParallel(ctx context.Context, files []string, result *IndexResult) (*IndexResult, error) {
	// Submit all files to the queue
	tasks := make([]*Task, len(files))
	for idx, path := range files {
		task, err := i.queue.Submit(ctx, path, TaskTypeFile)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to submit: %v", path, err))
			result.Failed++
			continue
		}
		tasks[idx] = task
	}

	// Wait for all tasks to complete
	for idx, task := range tasks {
		if task == nil {
			continue
		}

		select {
		case taskResult := <-task.Result:
			if taskResult.Error != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", files[idx], taskResult.Error))
				result.Failed++
			} else if taskResult.Document != nil {
				if taskResult.Document.IndexedAt != nil && taskResult.Document.IndexedAt.After(result.StartTime) {
					result.Indexed++
				} else {
					result.Skipped++
				}
				result.TotalChunks += taskResult.ChunksCount
			}
		case <-ctx.Done():
			result.Errors = append(result.Errors, fmt.Sprintf("%s: context cancelled", files[idx]))
			result.Failed++
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// DeleteDocument removes a document and its chunks from the index
func (i *Indexer) DeleteDocument(ctx context.Context, docID string) error {
	// Get chunks for this document
	chunks, err := i.store.GetChunksByDocument(docID)
	if err != nil {
		return err
	}
	
	// Delete from vector index
	for _, chunk := range chunks {
		if err := i.vectorIdx.Delete(chunk.ID); err != nil {
			// Log but continue
			fmt.Printf("failed to delete chunk from vector index: %v\n", err)
		}
	}
	
	// Delete chunks from store
	if err := i.store.DeleteChunksByDocument(docID); err != nil {
		return err
	}
	
	// Mark document as deleted
	return i.store.DeleteDocument(docID)
}

// ReindexDocument reindexes a document
func (i *Indexer) ReindexDocument(ctx context.Context, docID string) (*store.Document, error) {
	doc, err := i.store.GetDocument(docID)
	if err != nil {
		return nil, err
	}
	
	if err := i.DeleteDocument(ctx, docID); err != nil {
		return nil, err
	}
	
	return i.IndexFile(ctx, doc.Path)
}

// GetStats returns indexer statistics
func (i *Indexer) GetStats() map[string]interface{} {
	docCount, _ := i.store.CountDocuments()
	chunkCount, _ := i.store.CountChunks()
	vectorStats := i.vectorIdx.Stats()
	
	return map[string]interface{}{
		"documents": docCount,
		"chunks":    chunkCount,
		"vector":    vectorStats,
		"maxChunks": i.maxChunks,
	}
}

// parseFile parses a file and extracts text
func (i *Indexer) parseFile(path string) (string, error) {
	parser, err := i.parserReg.GetParserForPath(path)
	if err != nil {
		// Try to read as text anyway
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	
	// Check if it's a FileParser (PDF, DOCX) or regular Parser
	switch p := parser.(type) {
	case parsers.FileParser:
		// File parsers need the path
		return p.ParseFile(path)
	case parsers.Parser:
		// Regular parsers need a reader
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		return p.Parse(file)
	default:
		// Fallback: read as text
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// calculateChecksum calculates file checksum
func (i *Indexer) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// isSupportedFile checks if a file type is supported
func (i *Indexer) isSupportedFile(path string) bool {
	_, err := i.parserReg.GetParserForPath(path)
	return err == nil
}

// IndexResult holds the result of indexing
type IndexResult struct {
	Indexed     int
	Skipped     int
	Failed      int
	TotalChunks int
	Errors      []string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
}

// String returns a string representation of the result
func (r *IndexResult) String() string {
	return fmt.Sprintf(
		"Indexed: %d, Skipped: %d, Failed: %d, Total chunks: %d, Duration: %v",
		r.Indexed, r.Skipped, r.Failed, r.TotalChunks, r.Duration,
	)
}
