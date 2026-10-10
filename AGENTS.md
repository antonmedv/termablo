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

## Text and translations

Player-facing text lives in `locales/*.maml`, English (`en.maml`) the
source; never write it into Go code. Log lines go through
`g.say(col, "msg.key", args...)`, other text through `g.L.T(...)`, and
names through `itemNoun`, `monsterNoun`, `theRef`, `areaName`
(`i18n.go`). Measure text with `i18n.Width`, never `len` or rune counts.
A new or changed English line makes the translations stale: follow
`docs/translating.md` (`go run ./cmd/i18n status`, `prompt`, `merge`,
`check`, or `/translate`). Terms are decided in `locales/glossary.maml`.

## World and quest line

`docs/world.md`, `docs/world.png` and `docs/quests.md` are generated:
run `make docs` after adding or relinking a level or changing a quest.
The quest line is the `quests` table in `quest.go`, in order, each with
its boss's `Level`; `route.go` walks it, and the bot follows it.
