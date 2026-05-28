package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/smithy-go"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

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

// MetricsHandler fans out to Temporal and CloudWatch concurrently, merges
// results, and returns JSON. Responses are cached for a short TTL to avoid
// hammering both APIs when many browser tabs are polling simultaneously.
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
		)

		// Fan-out 1: Temporal — running and completed workflow counts.
		wg.Add(1)
		go func() {
			defer wg.Done()
			running, completed, err := fetchTemporalWorkflowCounts(ctx, tc)
			if err != nil {
				log.Printf("metrics: fetchTemporalWorkflowCounts: %v", err)
				return
			}
			mu.Lock()
			response.RunningWorkflows = running
			response.CompletedWorkflows = completed
			mu.Unlock()
		}()

		// Fan-out 2: CloudWatch — Lambda concurrent executions.
		// Degrades gracefully to 0 when running locally without AWS credentials.
		wg.Add(1)
		go func() {
			defer wg.Done()
			concurrency, err := fetchLambdaConcurrency(ctx, cwClient, lambdaFunctionName)
			if err != nil {
				log.Printf("metrics: fetchLambdaConcurrency: %v", err)
				return
			}
			mu.Lock()
			response.LambdaConcurrency = concurrency
			mu.Unlock()
		}()

		// Fan-out 3: Temporal — task queue backlog depth.
		wg.Add(1)
		go func() {
			defer wg.Done()
			backlog, err := fetchTaskQueueBacklog(ctx, tc)
			if err != nil {
				log.Printf("metrics: fetchTaskQueueBacklog: %v", err)
				return
			}
			mu.Lock()
			response.BacklogDepth = backlog
			mu.Unlock()
		}()

		wg.Wait()

		data, _ := json.Marshal(response)
		metricsCache.Set(data)
		w.Write(data)
	}
}

// fetchTemporalWorkflowCounts queries Temporal for running and completed
// workflow counts on the demo task queue.
func fetchTemporalWorkflowCounts(ctx context.Context, tc client.Client) (running, completed int64, err error) {
	runningResp, err := tc.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{
		Query: `TaskQueue="` + taskqueue.DemoTaskQueue + `" AND ExecutionStatus="Running"`,
	})
	if err != nil {
		return 0, 0, err
	}

	completedResp, err := tc.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{
		Query: `TaskQueue="` + taskqueue.DemoTaskQueue + `" AND ExecutionStatus="Completed"`,
	})
	if err != nil {
		return 0, 0, err
	}

	return runningResp.Count, completedResp.Count, nil
}

// fetchLambdaConcurrency queries CloudWatch for the ConcurrentExecutions metric
// over the last 60 seconds. Returns 0, nil when running locally without AWS
// credentials so the rest of the metrics response is unaffected.
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
		// Treat missing/invalid credentials as a graceful zero — expected in
		// local dev where no AWS credentials are configured.
		var ae smithy.APIError
		if errors.As(err, &ae) &&
			(ae.ErrorCode() == "AuthFailure" ||
				ae.ErrorCode() == "InvalidClientTokenId" ||
				ae.ErrorCode() == "ExpiredTokenException" ||
				ae.ErrorCode() == "NoCredentialProviders") {
			return 0, nil
		}
		return 0, err
	}

	if len(resp.Datapoints) == 0 {
		return 0, nil
	}

	// Return the most recent maximum datapoint.
	return aws.ToFloat64(resp.Datapoints[0].Maximum), nil
}

// fetchTaskQueueBacklog queries Temporal task queue stats for approximate
// backlog depth on the demo task queue.
func fetchTaskQueueBacklog(ctx context.Context, tc client.Client) (float64, error) {
	resp, err := tc.DescribeTaskQueue(ctx, taskqueue.DemoTaskQueue, 0)
	if err != nil {
		return 0, err
	}

	if resp.Stats != nil {
		return float64(resp.Stats.ApproximateBacklogCount), nil
	}
	return 0, nil
}
