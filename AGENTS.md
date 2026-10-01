# Termablo for agents

A Diablo-like roguelike for the terminal, in Go. `go test ./...` is the
suite; `make check` (fmt, lint, deadcode, tests) is what a commit passes.

## Balance pass

Game balance is tuned by self-play. Before and after changing combat,
loot, the economy or the bot (`bot.go`), follow `docs/balancing.md`:
`make eval` → `go run ./cmd/balance compare` → `log` → validate on fresh
seeds → `balance/REPORT.md`. The knobs are the registry in `rules.go`
(`make knobs`); the goals are `Goals` in `internal/balance/balance.go`.
Do not change `DefaultRules` without a validated run and a ledger entry.
