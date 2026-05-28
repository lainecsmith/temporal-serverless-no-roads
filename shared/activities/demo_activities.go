package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/temporalio/temporal-serverless-no-roads/shared/workflows"
)

// Activities holds any dependencies needed by activity implementations.
// Add things like HTTP clients, DB connections, etc. here as the demo grows.
type Activities struct{}

// ProcessSubmission is the activity that handles an audience submission.
// In the demo it just formats a greeting — but this is where you'd put real
// work if you wanted to show off something more elaborate.
func (a *Activities) ProcessSubmission(ctx context.Context, input workflows.DemoInput) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("ProcessSubmission activity started", "name", input.Name)

	result := fmt.Sprintf("Hello from Lambda, %s! Your workflow ran successfully.", input.Name)
	return result, nil
}
