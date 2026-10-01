# Balancing Termablo

Balance is tuned by self-play, by an agent running this loop. In Claude
Code type `/balance`; other agents reach this file through `AGENTS.md`.
The repo gives the instruments, the agent does the thinking. `balance/`
holds the shared state: the ledger, the best-known rules, the report and
the rules files; `balance/runs/` (the raw results) is ignored by git.

## Instruments

| what                                      | how                                                                                                                               |
|-------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| knobs (name, meaning, value, range, step) | `make knobs` — the registry in `rules.go`                                                                                         |
| one evaluation → JSON                     | `make eval OUT=balance/runs/NNN-slug.json [RULES=balance/rules/slug.json] [SEEDS=48] [FIRST=1]`                                   |
| human report only                         | `make report [RULES=..] [SEEDS=24]` (adds the reference-heroes table)                                                             |
| read a result                             | `go run ./cmd/balance show RUN.json`                                                                                              |
| paired comparison                         | `go run ./cmd/balance compare BASE.json RUN.json`                                                                                 |
| seeds needed                              | `go run ./cmd/balance seeds RUN.json`                                                                                             |
| record a decision                         | `go run ./cmd/balance log RUN.json -keep\|-revert\|-baseline -change ".." -why ".." [-base BASE.json]`                            |
| ledger                                    | `go run ./cmd/balance ls`, `balance/LEDGER.md`, `balance/ledger.jsonl`                                                            |
| best known                                | `balance/best.json` (rules), `balance/best-run.json` (its result)                                                                 |
| read one run's end                        | `BOTTRACE=fighter:7:60 go test -count=1 -v -run BotTrace .` (last 60 log lines with life, potions, bot state; `BOTRULES` applies) |
| watch a run                               | `go run . -bot fighter -seed 7`                                                                                                   |

A rules file is a JSON overlay on `DefaultRules`: `{"HpLin": 0.3}`. Unknown
knobs fail, out-of-range values warn. The goals (bands and weights) are
`Goals` in `internal/balance/balance.go`; the score is the weighted
distance outside them, lower is better, 0 is all in band.

## The loop

1. **Inspect.** `make knobs`; read `rules.go`, `bot.go`, the formulas the
   knob touches. Take a baseline: `make eval OUT=balance/runs/000-baseline.json SEEDS=48`,
   `log -baseline`.
2. **Analyze.** `show` the result. Largest misses first. For each, name
   the mechanism (which formula, which monster, which bot habit) before
   touching a number. Watch a seed with `-bot` when the numbers do not
   explain themselves.
3. **Is it the bot?** A miss caused by the bot not using a mechanic
   (never casts Nova, ignores a shop service, walks past the boss) is a
   bot bug: fix `bot.go` first, re-baseline (`log -keep -change "bot: .."`),
   then tune. Never tune rules around a bot blind spot.
4. **Modify.** One hypothesis per run: write `balance/rules/NNN-slug.json`
   with only the knobs that changed, say what should move and by how much.
   Code changes (formula shapes, new mechanics) are allowed when a knob
   cannot express the fix; they get their own ledger entry.
5. **Validate.** `make eval` on the same seeds as the base, then
   `compare BASE RUN`. Keep only when the score interval says "B better"
   or when the targeted miss moves as predicted and nothing else gets
   surely worse. Otherwise `-revert`. Log every run, kept or not.
6. **Regressions.** Compare against the best-known run, not just the last.
   A kept change that later looks worse under more seeds is reverted with
   its own entry.
7. **Confidence.** 48 seeds ≈ ±3 score, ±10 pts on a boss rate. Before
   declaring a winner, re-run the pick on fresh seeds
   (`FIRST=1001 SEEDS=48`) and compare: the pick must hold there.
8. **Break it.** Look for exploits with the data: gold that piles up
   (`*.gold`, `goldHeld`), a shop loop (reroll/gamble sinks vs sells),
   farming (kills per level, turns per level), a build that never dies in
   an area, a mechanic the bot abuses. Add a bot variant or a probe test
   when the standard policies cannot show it.
9. **Stop** when the score stops improving beyond its interval for three
   runs, or the remaining misses are bot taste or design questions for the
   human.

## Lessons from the 2026-09-30 pass

- 48 seeds ≈ ±3 score and ±10 points on a boss rate; 96 seeds and the
  paired table are what settle a knob. Expect one false "sure" in ~30
  comparisons; look for the mechanism, not just the star.
- A hard fail on stuck runs is usually a bot loop, not the rules. Known
  signatures: a repeated bot line with the same turn number (an action
  the game refused without spending a turn); "to the frontier" / "auto-
  explore" alternating between two cells (two explorers disagreeing); a
  long loot sweep or stairs walk called stuck (progress accounting).
  Residual, about 1%: "nothing to do here" with the stairs unseen after
  a full exploration.
- The 1v1 bands can all sit in range while every run dies at one depth:
  packs decide. `packLive` and `fight` are the pack-aware numbers.
- The caster is limited by mana per pool (kills per pool fell from 3 to
  1.7 with depth); the fighter by melee against champion packs in open
  caves. Shared hero stats (ManaPerLvl, HPPerLvl, potions) move the
  caster and not the fighter; the fighter's own levers are StrDiv, melee
  to-hit and monster armor, and only StrDiv measured.
- Gambling is the fighter's gear pipeline: any sink price or sell cut
  that meets the gold band costs the fighter levels (sure). The band
  0.8–1.5 gambles a level conflicts with that under these rules.
- Champions are what the fighter farms for XP and drops: fewer or weaker
  champions help the caster, not the fighter, and widen the gap.

## Report

Write `balance/REPORT.md`: problems found, bot changes, every kept change
with its reason and its numbers, simulations run, how the key metrics
moved (baseline → final, fighter vs caster), regressions reverted, remaining
concerns, the recommended configuration (`balance/best.json` as a
`DefaultRules` diff), and the code changes for review (`git diff --stat`).
Do not commit; the human applies the pick.
