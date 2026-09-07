.PHONY: build test integration race vet generate dev-db smoke up down
build:
	mkdir -p bin
	go build -o bin/ ./cmd/...
test:
	go test ./internal/...
integration:
	test -n "$(TEST_DATABASE_URL)"
	go test ./internal/store ./internal/service -count=1
race:
	go test -race ./internal/...
vet:
	go vet ./...
generate:
	python3 scripts/generate_idl.py
	go run github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.3 -module ai-travel idl/travel.thrift
	go run github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.3 -module ai-travel idl/planner.thrift
	python3 scripts/generate_rpc.py
	gofmt -w internal/rpc
dev-db:
	go run ./cmd/devdb
smoke: build
	python3 scripts/smoke.py
up:
	docker compose up --build -d
down:
	docker compose down
