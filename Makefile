.PHONY: run build check fmt lint deadcode test test-short fuzz bench shots og zones docs report eval hearth knobs i18n

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
	@out=$$($(GOBIN)/deadcode -test ./...); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

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

# GitHub social preview (Settings → Social preview), 1280×640
og:
	SHOTDIR=$(CURDIR)/demo go test -count=1 -run ShotOG .

# README region postcards, demo/zones.png
zones:
	SHOTDIR=$(CURDIR)/demo go test -count=1 -run ShotZones .

# Generated docs: the world map (docs/world.md, docs/world.png) and the
# quest line the bot walks (docs/quests.md)
docs:
	SHOTDIR=$(CURDIR)/docs go test -count=1 -run 'ShotWorld|DocQuests' .

# the bot report: both builds over SEEDS seeds, the reference heroes
# table. RULES=file.json lays knobs over the defaults (rules.go).
SEEDS ?= 24
FIRST ?= 1
report:
	BOTSEEDS=$(SEEDS) BOTFIRST=$(FIRST) BOTRULES=$(RULES) go test -count=1 -v -run 'BotBalance|RefHeroes' . | grep -v "^=== RUN"

# one balance evaluation: the report plus its rows, metrics and score as
# JSON for cmd/balance. make eval OUT=balance/runs/x.json RULES=x.json
eval:
	@test -n "$(OUT)" || { echo "make eval OUT=balance/runs/NAME.json [RULES=x.json SEEDS=48 FIRST=1]"; exit 2; }
	BOTSEEDS=$(SEEDS) BOTFIRST=$(FIRST) BOTRULES=$(RULES) BOTOUT=$(OUT) BOTNAME=$(notdir $(basename $(OUT))) go test -count=1 -v -run 'BotBalance$$' . | grep -v "^=== RUN"

# the final fight on its own, which no eval run reaches: a par hero from
# the Hearth stairs, on the floor as generated and on a duel floor.
# make hearth [SEEDS=48 FIRST=1 RULES=x.json]
hearth:
	BOTHEARTH=$(SEEDS) BOTHEARTHFIRST=$(FIRST) BOTRULES=$(RULES) go test -count=1 -v -run 'HearthTrial$$' . | grep -E "won|stuck:"

# the translations: coverage, errors and warnings (docs/translating.md)
i18n:
	go run ./cmd/i18n check

# the knob registry: every tunable with its range, step and meaning
knobs:
	@go test -count=1 -v -run 'Knobs$$' . | grep "eval_test" | sed 's/^ *eval_test.go:[0-9]*: //'
