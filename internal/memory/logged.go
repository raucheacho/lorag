package memory

import (
	"time"

	"github.com/lorag/lorag/internal/activity"
)

// RecallLogged runs a recall and records it in the activity log, so agent
// logs show memory lookups next to searches.
func (m *Service) RecallLogged(log *activity.Log, req RecallRequest, src activity.Source, client string) (*RecallResult, error) {
	start := time.Now()
	res, err := m.Recall(req)
	if log != nil {
		q := activity.Query{
			Source: src, Client: client, Tool: "memory_recall", Mode: "memory",
			Text: req.Query, TopK: max(req.K, 1), DurationMs: float64(time.Since(start).Microseconds()) / 1000,
		}
		if q.Text == "" {
			q.Text = req.Error
		}
		if err != nil {
			q.Error = err.Error()
		} else {
			for _, mt := range res.Matches {
				q.Hits = append(q.Hits, activity.Hit{
					ChunkID: mt.Entry.ID, Filename: mt.Entry.Problem, Path: "memory/" + mt.Entry.Space,
					Score: float32(mt.Entry.Confidence()),
				})
			}
		}
		log.Record(q)
	}
	return res, err
}
