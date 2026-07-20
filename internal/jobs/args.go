// Package jobs defines River job argument types shared between the API layer
// (which enqueues jobs) and the worker layer (which processes them).
package jobs

// NLPProcessingArgs is the payload for the nlp_processing River job.
// OrgID carries the tenant so the worker can open an RLS-scoped transaction
// that obeys the same isolation boundaries as the request that enqueued it.
type NLPProcessingArgs struct {
	DailyUpdateID int64  `json:"daily_update_id"`
	OrgID         string `json:"org_id"`
}

// Kind returns the unique River job kind discriminator stored in river_job.kind.
func (NLPProcessingArgs) Kind() string { return "nlp_processing" }
