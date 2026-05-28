package main

import (
	"go.temporal.io/sdk/contrib/aws/lambdaworker"
	"go.temporal.io/sdk/worker"

	"github.com/temporalio/temporal-serverless-no-roads/shared/activities"
	"github.com/temporalio/temporal-serverless-no-roads/shared/taskqueue"
	"github.com/temporalio/temporal-serverless-no-roads/shared/workflows"
)

func main() {
	lambdaworker.RunWorker(
		worker.WorkerDeploymentVersion{
			DeploymentName: "serverless-demo",
			BuildID:        "v1.0",
		},
		func(opts *lambdaworker.Options) error {
			opts.TaskQueue = taskqueue.DemoTaskQueue

			opts.RegisterWorkflow(workflows.DemoWorkflow)
			opts.RegisterActivity(&activities.Activities{})

			return nil
		},
	)
}
