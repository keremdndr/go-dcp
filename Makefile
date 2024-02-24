.PHONY: default test

default: init

init:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2
	go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@v0.15.0

clean:
	rm -rf ./build

linter:
	fieldalignment -fix ./...
	golangci-lint run -c .golangci.yml --timeout=5m -v --fix

lint:
	golangci-lint run -c .golangci.yml --timeout=5m -v

test:
	go test ./... -bench .

create-cluster:
	bash scripts/create_cluster.sh --version $(version)

delete-cluster:
	bash scripts/delete_cluster.sh

docker-build:
	docker build --progress=plain -t docker.io/trendyoltech/dcp .