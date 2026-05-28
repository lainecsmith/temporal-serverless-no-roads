.PHONY: build deploy

FUNCTION_NAME ?= serverless-demo-worker

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap .

deploy: build
	./deploy-lambda.sh $(FUNCTION_NAME)
