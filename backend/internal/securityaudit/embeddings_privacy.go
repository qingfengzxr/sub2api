package securityaudit

import "context"

// Observe inline so embedding inputs never enter the persistent async payload store.
// Observe mode retains its allow-next-stage semantics even if the detector flags input.
func (c *Coordinator) checkEmbeddingsObservation(ctx context.Context, req Request) Decision {
	if _, err := c.prompt.Evaluate(ctx, req); err != nil {
		LogWarn(EventGuardFailed, map[string]any{"request_id": req.RequestID, "error_code": ErrorCodeUnavailable})
	}
	legacy, _ := c.checkLegacy(ctx, req)
	return prioritize(legacy, nil)
}
