package workflows

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// DemoInput is the input to the DemoWorkflow — just the submitter's name.
type DemoInput struct {
	Name string `json:"name"`
}

// DemoOutput is returned when the workflow completes.
type DemoOutput struct {
	Message string `json:"message"`
}

// DemoWorkflow is the workflow that gets triggered by each audience submission.
// It runs a short processing activity, then sleeps to keep it "running" long
// enough to be visible on the dashboard during the webinar demo.
func DemoWorkflow(ctx workflow.Context, input DemoInput) (DemoOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("DemoWorkflow started", "name", input.Name)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: run the "processing" activity
	var result string
	err := workflow.ExecuteActivity(ctx, "ProcessSubmission", input).Get(ctx, &result)
	if err != nil {
		return DemoOutput{}, err
	}

	// Step 2: artificial sleep — keeps the workflow "running" so the dashboard
	// shows meaningful concurrency numbers during the live demo.
	// Tune this to match your desired demo window (default: 8 seconds).
	err = workflow.Sleep(ctx, 8*time.Second)
	if err != nil {
		return DemoOutput{}, err
	}

	logger.Info("DemoWorkflow completed", "name", input.Name)
	return DemoOutput{Message: result}, nil
}
