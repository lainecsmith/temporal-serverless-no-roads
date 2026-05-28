# temporal-serverless-no-roads

> *We Don't Need Workers Where We're Going*

A live audience-participation demo for Temporal's Serverless Workers feature. Attendees submit their name via a web UI to trigger real workflow executions, and watch Lambda invocations, task queue backlog, and workflow counts update in real time.

## Repo structure

```
temporal-serverless-no-roads/
├── shared/               # Shared Go module — workflow, activity, task queue name
├── lambda-worker/        # Deployable 1: Lambda worker (Go)
├── demo-app/             # Deployable 2: HTTP server — UI + API (Go)
│   ├── api/              # /api/submit and /api/metrics handlers
│   ├── cache/            # Short-TTL metrics cache
│   ├── frontend/         # Embedded HTML UI
│   ├── localworker/      # Long-polling worker for local dev (not deployed)
│   ├── middleware/       # Per-IP rate limiter
│   └── k8s/              # Kubernetes manifests for EKS deployment
├── go.work               # Go workspace — ties all three modules together
└── README.md
```

---

## Running locally

Local dev uses the Temporal CLI dev server in place of Temporal Cloud, and a
standard Go worker in place of the Lambda worker. No AWS account required.

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Temporal CLI](https://docs.temporal.io/cli#installation)

```bash
# macOS
brew install temporal

# Linux — check your arch and download from:
# https://temporal.download/cli/archive/latest?platform=linux&arch=amd64
```

### 1. Resolve dependencies

Run `go mod tidy` in each module (the workspace root has no `go.mod`, so you
need to do this per-module):

```bash
cd shared        && go mod tidy && cd ..
cd lambda-worker && go mod tidy && cd ..
cd demo-app      && go get go.temporal.io/api && go mod tidy && cd ..
```

> **Note:** `go.temporal.io/api` is a transitive dependency of the SDK but
> needs to be explicit in `demo-app/go.mod` because `metrics.go` imports
> `go.temporal.io/api/workflowservice/v1` directly for `CountWorkflowExecutionsRequest`.

Then sync the workspace:

```bash
go work sync
```

### 2. Start the Temporal dev server

In a dedicated terminal — leave this running:

```bash
temporal server start-dev
```

This starts a local Temporal cluster at `localhost:7233` with the Web UI at
[http://localhost:8233](http://localhost:8233). No auth, no TLS — perfect for
local dev.

### 3. Start a local worker

The Lambda worker uses `lambdaworker.RunWorker`, which is designed for Lambda
invocations and exits after each task batch — not ideal for local iteration.
Instead, run the long-polling local worker from `demo-app/localworker/`:

```bash
cd demo-app
go run ./localworker/main.go
```

This registers the same workflows and activities from `shared/` against your
local Temporal dev server.

### 4. Start the demo app

In another terminal:

```bash
cd demo-app
LAMBDA_FUNCTION_NAME=local go run .
```

`LAMBDA_FUNCTION_NAME` is required by the metrics handler. Setting it to any
non-empty string (e.g. `local`) is fine for local dev — CloudWatch calls will
no-op gracefully when no AWS credentials are present, and the Lambda concurrency
stat will show `0`.

The demo app will be available at [http://localhost:8080](http://localhost:8080).

### 5. Try it out

Open [http://localhost:8080](http://localhost:8080), enter a name, and click
**Start workflow**. You should see:

- The running workflow count increment
- The activity feed populate with your submission
- The task queue backlog briefly spike then drain
- The Temporal Web UI at [http://localhost:8233](http://localhost:8233) show
  the workflow execution in real time

### Local environment summary

| Terminal | Command | Purpose |
|---|---|---|
| 1 | `temporal server start-dev` | Local Temporal cluster + Web UI |
| 2 | `cd demo-app && go run ./localworker/main.go` | Long-polling worker |
| 3 | `cd demo-app && LAMBDA_FUNCTION_NAME=local go run .` | Demo app server |

---

## Deploying to AWS + EKS

See the per-component READMEs (coming soon) for full deployment instructions.
High-level steps:

1. **Lambda worker** — run `./lambda-worker/mk-iam-role.sh` to create the IAM
   role Temporal Cloud assumes, then `make deploy` to build and push the
   function code.
2. **Temporal Cloud** — configure the serverless worker deployment version in
   your namespace settings, pointing at the IAM role ARN and Lambda function ARN.
3. **Demo app** — build and push the Docker image, then apply the k8s manifests:
   ```bash
   docker build -t <your-ecr-repo>/demo-app:latest -f demo-app/Dockerfile .
   docker push <your-ecr-repo>/demo-app:latest
   kubectl apply -f demo-app/k8s/
   ```

---

## Configuration reference

### Demo app environment variables

| Variable | Required | Description |
|---|---|---|
| `LAMBDA_FUNCTION_NAME` | Yes | Lambda function name for CloudWatch metrics lookup |
| `TEMPORAL_ADDRESS` | No | Temporal server address (default: `localhost:7233`) |
| `TEMPORAL_NAMESPACE` | No | Temporal namespace (default: `default`) |
| `TEMPORAL_TLS_CERT` | No | Path to mTLS client cert (Temporal Cloud) |
| `TEMPORAL_TLS_KEY` | No | Path to mTLS client key (Temporal Cloud) |

### Tuning the demo

The workflow sleep duration controls how long executions stay "running" on the
dashboard — longer means more overlap and more dramatic scaling visuals. Edit
`shared/workflows/demo_workflow.go`:

```go
// Tune this to match your desired demo window (default: 8 seconds)
err = workflow.Sleep(ctx, 8*time.Second)
```

A value of 8–12 seconds works well for a live webinar — long enough for the
audience to see the counters and charts respond, short enough that the board
doesn't fill up with stale running workflows.
