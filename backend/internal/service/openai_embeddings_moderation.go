package service

// Inline observations retain observe-mode overload behavior without retaining request bodies in a queue.
func (s *ContentModerationService) acquireEmbeddingsObservation(workerCount int) bool {
	if workerCount <= 0 {
		workerCount = defaultContentModerationWorkerCount
	}
	if workerCount > maxContentModerationWorkerCount {
		workerCount = maxContentModerationWorkerCount
	}
	for {
		active := s.embeddingsObserveActive.Load()
		if active >= int64(workerCount) {
			return false
		}
		if s.embeddingsObserveActive.CompareAndSwap(active, active+1) {
			return true
		}
	}
}
