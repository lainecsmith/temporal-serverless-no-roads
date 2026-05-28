package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"go.temporal.io/sdk/client"
	"go.temporal.io/api/workflowservice/v1"

	"github.com/temporalio/temporal-serverless-no-roads/demo-app/cache"
	"github.com/temporalio/temporal-serverless-no-roads/shared/taskqueue"
)

// MetricsResponse is the JSON shape the frontend polls for.
type MetricsResponse struct {
	RunningWorkflows   int64   `json:"runningWorkflows"`
	CompletedWorkflows int64   `json:"completedWorkflows"`
	LambdaConcurrency  float64 `json:"lambdaConcurrency"`
	BacklogDepth       float64 `json:"backlogDepth"`
}

// MetricsHandler fans out to Temporal Cloud metrics and CloudWatch
// concurrently, merges results, and returns JSON. Responses are cached for
// a short TTL to avoid hammering both APIs when many browser tabs are polling.
func MetricsHandler(
	tc client.Client,
	cwClient *cloudwatch.Client,
	metricsCache *cache.MetricsCache,
	lambdaFunctionName string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		// Return cached response if still fresh.
		if cached, ok := metricsCache.Get(); ok {
			w.Write(cached)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			response MetricsResponse
			errs     []error
		)

		// Fan-out 1: Temporal Cloud — running and completed workflow counts.
		wg.Add(1)
		go func() {
			defer wg.Done()
			running, completed, err := fetchTemporalWorkflowCounts(ctx, tc)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			response.RunningWorkflows = running
			response.CompletedWorkflows = completed
		}()

		// Fan-out 2: CloudWatch — Lambda concurrent executions.
		wg.Add(1)
		go func() {
			defer wg.Done()
			concurrency, err := fetchLambdaConcurrency(ctx, cwClient, lambdaFunctionName)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			response.LambdaConcurrency = concurrency
		}()

		// Fan-out 3: Temporal Cloud metrics — task queue backlog depth.
		wg.Add(1)
		go func() {
			defer wg.Done()
			backlog, err := fetchTaskQueueBacklog(ctx, tc)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			response.BacklogDepth = backlog
		}()

		wg.Wait()

		data, _ := json.Marshal(response)
		metricsCache.Set(data)
		w.Write(data)
	}
}

// fetchTemporalWorkflowCounts queries Temporal Cloud for running and completed
// workflow counts on the demo task queue via the List API.
func fetchTemporalWorkflowCounts(ctx context.Context, tc client.Client) (running, completed int64, err error) {
	// Query running workflows
	runningResp, err := tc.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{
		Query: `TaskQueue="` + taskqueue.DemoTaskQueue + `" AND ExecutionStatus="Running"`,
	})
	if err != nil {
		return 0, 0, err
	}

	// Query completed workflows
	completedResp, err := tc.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{
		Query: `TaskQueue="` + taskqueue.DemoTaskQueue + `" AND ExecutionStatus="Completed"`,
	})
	if err != nil {
		return 0, 0, err
	}

	return runningResp.Count, completedResp.Count, nil
}

// fetchLambdaConcurrency queries CloudWatch for the ConcurrentExecutions metric
// for the Lambda function over the last 60 seconds.
func fetchLambdaConcurrency(ctx context.Context, cwClient *cloudwatch.Client, functionName string) (float64, error) {
	now := time.Now()
	resp, err := cwClient.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/Lambda"),
		MetricName: aws.String("ConcurrentExecutions"),
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("FunctionName"),
				Value: aws.String(functionName),
			},
		},
		StartTime:  aws.Time(now.Add(-60 * time.Second)),
		EndTime:    aws.Time(now),
		Period:     aws.Int32(60),
		Statistics: []cwtypes.Statistic{cwtypes.StatisticMaximum},
	})
	if err != nil {
		return 0, err
	}

	if len(resp.Datapoints) == 0 {
		return 0, nil
	}

	// Return the most recent maximum datapoint.
	return aws.ToFloat64(resp.Datapoints[0].Maximum), nil
}

// fetchTaskQueueBacklog queries the Temporal Cloud task queue stats for
// approximate backlog depth on the demo task queue.
func fetchTaskQueueBacklog(ctx context.Context, tc client.Client) (float64, error) {
	resp, err := tc.DescribeTaskQueue(ctx, taskqueue.DemoTaskQueue, 0)
	if err != nil {
		return 0, err
	}

	// TaskQueueStats.ApproximateBacklogCount is the field we want.
	if resp.Stats != nil {
		return float64(resp.Stats.ApproximateBacklogCount), nil
	}
	return 0, nil
}
