package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lorag/lorag/internal/core/store"
)

// Task represents an indexing task
type Task struct {
	ID        string
	Path      string
	Type      TaskType
	Context   context.Context
	Result    chan TaskResult
	StartTime time.Time
}

// TaskType represents the type of indexing task
type TaskType int

const (
	TaskTypeFile TaskType = iota
	TaskTypeDirectory
)

// TaskResult represents the result of an indexing task
type TaskResult struct {
	TaskID      string
	Document    *store.Document
	Error       error
	Duration    time.Duration
	ChunksCount int
}

// Queue manages indexing tasks with a worker pool
type Queue struct {
	indexer    *Indexer
	tasks      chan *Task
	workers    int
	wg         sync.WaitGroup
	mu         sync.RWMutex
	running    map[string]*Task
	completed  map[string]*TaskResult
	maxQueue   int
}

// NewQueue creates a new indexing queue
func NewQueue(indexer *Indexer, workers, maxQueue int) *Queue {
	if workers <= 0 {
		workers = 3 // Default 3 concurrent workers
	}
	if maxQueue <= 0 {
		maxQueue = 100 // Default max queue size
	}

	q := &Queue{
		indexer:   indexer,
		tasks:     make(chan *Task, maxQueue),
		workers:   workers,
		running:   make(map[string]*Task),
		completed: make(map[string]*TaskResult),
		maxQueue:  maxQueue,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	return q
}

// worker processes tasks from the queue
func (q *Queue) worker(id int) {
	defer q.wg.Done()

	for task := range q.tasks {
		// Mark as running
		q.mu.Lock()
		q.running[task.ID] = task
		q.mu.Unlock()

		// Process task (no timeout - let it take as long as needed)
		result := q.processTask(task)

		// Mark as completed
		q.mu.Lock()
		delete(q.running, task.ID)
		q.completed[task.ID] = &result
		q.mu.Unlock()

		// Send result back
		select {
		case task.Result <- result:
		case <-time.After(5 * time.Second):
			// Result channel blocked, log and continue
			fmt.Printf("Warning: task %s result channel blocked\n", task.ID)
		}
	}
}

// processTask processes a single task
func (q *Queue) processTask(task *Task) TaskResult {
	start := time.Now()

	var doc *store.Document
	var err error
	var chunksCount int

	switch task.Type {
	case TaskTypeFile:
		doc, err = q.indexer.IndexFile(task.Context, task.Path)
		if doc != nil {
			chunksCount = doc.ChunkCount
		}
	case TaskTypeDirectory:
		result, indexErr := q.indexer.IndexDirectory(task.Context, task.Path, true)
		err = indexErr
		if result != nil {
			chunksCount = result.TotalChunks
			// Create a pseudo-document for directory results
			doc = &store.Document{
				ID:         task.ID,
				Path:       task.Path,
				Filename:   "[DIRECTORY]",
				ChunkCount: result.TotalChunks,
			}
		}
	}

	duration := time.Since(start)

	return TaskResult{
		TaskID:      task.ID,
		Document:    doc,
		Error:       err,
		Duration:    duration,
		ChunksCount: chunksCount,
	}
}

// Submit adds a task to the queue and returns immediately
func (q *Queue) Submit(ctx context.Context, path string, taskType TaskType) (*Task, error) {
	task := &Task{
		ID:        generateTaskID(),
		Path:      path,
		Type:      taskType,
		Context:   ctx,
		Result:    make(chan TaskResult, 1), // Buffered to not block
		StartTime: time.Now(),
	}

	select {
	case q.tasks <- task:
		return task, nil
	case <-time.After(100 * time.Millisecond):
		return nil, fmt.Errorf("queue is full (max %d tasks)", q.maxQueue)
	}
}

// SubmitAndWait adds a task and waits for completion
func (q *Queue) SubmitAndWait(ctx context.Context, path string, taskType TaskType) (*TaskResult, error) {
	task, err := q.Submit(ctx, path, taskType)
	if err != nil {
		return nil, err
	}

	select {
	case result := <-task.Result:
		return &result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetRunning returns currently running tasks
func (q *Queue) GetRunning() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*Task, 0, len(q.running))
	for _, task := range q.running {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetCompleted returns completed task results
func (q *Queue) GetCompleted(taskID string) (*TaskResult, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result, ok := q.completed[taskID]
	return result, ok
}

// GetStats returns queue statistics
func (q *Queue) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return map[string]interface{}{
		"workers":      q.workers,
		"running":      len(q.running),
		"completed":    len(q.completed),
		"queue_depth":  len(q.tasks),
		"max_queue":    q.maxQueue,
	}
}

// Stop gracefully shuts down the queue
func (q *Queue) Stop() {
	close(q.tasks)
	q.wg.Wait()
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// DirectoryBatchResult holds results for batch directory indexing
type DirectoryBatchResult struct {
	Results   []TaskResult
	TotalTime time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// SubmitBatch submits multiple files and waits for all to complete
func (q *Queue) SubmitBatch(ctx context.Context, paths []string, taskType TaskType) *DirectoryBatchResult {
	start := time.Now()
	
	tasks := make([]*Task, len(paths))
	for i, path := range paths {
		task, _ := q.Submit(ctx, path, taskType)
		tasks[i] = task
	}

	// Wait for all results
	results := make([]TaskResult, len(tasks))
	for i, task := range tasks {
		if task != nil {
			select {
			case result := <-task.Result:
				results[i] = result
			case <-ctx.Done():
				results[i] = TaskResult{
					TaskID: task.ID,
					Error:  ctx.Err(),
				}
			}
		}
	}

	return &DirectoryBatchResult{
		Results:   results,
		TotalTime: time.Since(start),
		StartTime: start,
		EndTime:   time.Now(),
	}
}
