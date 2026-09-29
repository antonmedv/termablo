.PHONY: run build check fmt lint deadcode test test-short fuzz bench shots report

GOBIN := $(shell go env GOPATH)/bin

run:
	go run .

build:
	go build -o termablo .

# everything a commit should pass
check: fmt lint deadcode test

fmt:
	gofmt -l -w .

lint:
	golangci-lint run ./...

deadcode:
	@out=$$($(GOBIN)/deadcode -test .); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

test:
	go test ./...

test-short:
	go test -short ./...

fuzz:
	go test -run "^$$" -fuzz FuzzKeys -fuzztime 60s .

# save a baseline, change code, run again, compare: make bench; benchstat old.txt new.txt
bench:
	go test -run "^$$" -bench . -count 6 . | tee bench.txt

shots:
	mkdir -p shots && SHOTDIR=$(CURDIR)/shots go test -count=1 -run Shot .

report:
	go test -count=1 -v -run BotBalance . | grep -v "^=== RUN"
