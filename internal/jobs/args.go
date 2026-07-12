// Package jobs defines River job argument types shared between the API layer
// (which enqueues jobs) and the worker layer (which processes them).
package jobs

// NLPProcessingArgs is the payload for the nlp_processing River job.
type NLPProcessingArgs struct {
	DailyUpdateID int64 `json:"daily_update_id"`
}

// Kind returns the unique River job kind discriminator stored in river_job.kind.
func (NLPProcessingArgs) Kind() string { return "nlp_processing" }
