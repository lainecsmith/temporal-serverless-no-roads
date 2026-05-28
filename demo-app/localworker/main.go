package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/temporalio/temporal-serverless-no-roads/shared/activities"
	"github.com/temporalio/temporal-serverless-no-roads/shared/taskqueue"
	"github.com/temporalio/temporal-serverless-no-roads/shared/workflows"
)

func main() {
	c, err := client.Dial(client.Options{
		// Defaults to localhost:7233 / namespace "default" — matches
		// what `temporal server start-dev` provides out of the box.
	})
	if err != nil {
		log.Fatalln("failed to create Temporal client:", err)
	}
	defer c.Close()

	w := worker.New(c, taskqueue.DemoTaskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.DemoWorkflow)
	w.RegisterActivity(&activities.Activities{})

	log.Printf("local worker started, polling task queue: %s", taskqueue.DemoTaskQueue)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("worker error:", err)
	}
}
