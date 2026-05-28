package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"

	"go.temporal.io/sdk/client"

	"github.com/temporalio/temporal-serverless-no-roads/shared/taskqueue"
	"github.com/temporalio/temporal-serverless-no-roads/shared/workflows"
)

// SubmitRequest is the JSON body expected from the frontend.
type SubmitRequest struct {
	Name string `json:"name"`
}

// SubmitResponse is returned to the frontend on success.
type SubmitResponse struct {
	WorkflowID string `json:"workflowId"`
	Message    string `json:"message"`
}

// SubmitHandler returns an http.HandlerFunc that starts a DemoWorkflow for
// each audience submission.
func SubmitHandler(tc client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, "invalid request body — 'name' is required", http.StatusBadRequest)
			return
		}

		// Use the name as a human-readable workflow ID prefix so it shows up
		// nicely in the Temporal Cloud UI during the demo.
		workflowID := "demo-" + sanitizeName(req.Name) + "-" + shortID()

		opts := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: taskqueue.DemoTaskQueue,
		}

		we, err := tc.ExecuteWorkflow(r.Context(), opts, workflows.DemoWorkflow, workflows.DemoInput{
			Name: req.Name,
		})
		if err != nil {
			http.Error(w, "failed to start workflow: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SubmitResponse{
			WorkflowID: we.GetID(),
			Message:    "workflow started",
		})
	}
}

// sanitizeName strips characters that aren't safe in a Temporal workflow ID.
func sanitizeName(name string) string {
	out := make([]byte, 0, len(name))
	for _, c := range name {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_':
			out = append(out, byte(c))
		case c == ' ':
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "anon"
	}
	return string(out)
}

// shortID returns a short random hex string for workflow ID uniqueness.
func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
