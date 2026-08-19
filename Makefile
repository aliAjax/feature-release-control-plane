.PHONY: fmt vet test build run smoke count stop
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
vet:
	go vet ./...
test:
	go test ./...
build:
	go build ./cmd/controlplane ./cmd/worker
run:
	go run ./cmd/controlplane
smoke:
	./build/smoke.sh
count:
	find . -name '*.go' -not -name '*_test.go' -not -path './vendor/*' -print0 | xargs -0 wc -l | tail -1
stop:
	docker compose down --remove-orphans
