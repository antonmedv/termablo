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
| the final fight (no eval run reaches it)  | `make hearth [SEEDS=48] [FIRST=1] [RULES=..]`; one run's log: `BOTHEARTH=24 BOTHEARTHLOG=fighter:3:true go test -count=1 -v -run HearthTrial .` |
| the Cinder Prior, or any floor on its own | `make kindling [SEEDS=48] [AT=abyss1] [KIT=abyss2] [GOLD=3000] [RULES=..]`: a par hero as the bot arrives at the Kindling; one run's log with its bot states: `BOTKINDLING=4 BOTKINDLINGLOG=fighter:3:false go test -count=1 -v -run KindlingTrial .` |

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

## Lessons from the 2026-10-01 pass

- Trace before tuning. Every grotto death read the same way: the bot
  had backed into a 3-sided nook, three attackers filled it, and the
  portal it read with an empty belt opened behind them. All of the
  previous pass's grotto numbers were measured on that trap; once the
  escape worked (score 18.0 → 13.2 on seeds 1–96, 16.1 → 13.6 on fresh
  seeds, both sure) the same knobs measured differently. ChampHp 1.8 now
  reached the fighter too.
- A bot fix that reads well can still be wrong: a pre-emptive portal at
  belt ≤ 3 fired from the first fight (start kit: 3 potions, 1 scroll)
  and drained the early game; a leave line that rose with attackers cut
  trips short. Measure each half on its own.
- The residual stuck class is two goals a step apart: an enemy that
  shows from one cell only and loot the other way, or a path that goes
  around a monster only from the cells it is visible from. Fix the
  memory (monsters seen are remembered like the map), keep a generic
  wobble guard, and never chase what is out of view: that was surely
  worse for both builds.
- With the escape working, grotto deaths come broke: 0 scrolls, ~30 g,
  after 3–5 kills a belt and a gamble every trip. Income levers
  (GoldPerLvl) move the fighter but the gold band takes it back; the
  gold band is now at odds with progress twice over.
- What moves the grotto wall is how many packs join a fight (WakeRange
  16 → 10, both builds, both seed sets), not how strong a champion is:
  one champion leading normals left deepest unchanged and only cut
  income. The king band's ceiling (0.85) and the gold band then absorb
  the gain, so the score stays flat while deaths move a level deeper.
  The next lever is the crypt, which the pack bands also want harder.

## Lessons from the 2026-10-10 pass

- Probe a death class before fixing it. A test over all 192 runs that
  reads the end state (portal on the level, who stands on it, what the
  bot was doing) found 31 deaths with a monster on the portal the bot
  was walking to, 20 of them with a scroll in hand; the fix (read a
  fresh one) was sure for the caster and nothing for the fighter.
- A floor trial is the cheap instrument for a wall: a par hero dropped
  on one floor, 48 seeds, 20 seconds. Dropped on the Kindling it dies
  48/48 with the Prior untouched, and beats him 46/48 in the duel; on
  abyss1 and abyss2 it dies 46–47/48, on the Sanctum chapel 1/48, on
  grotto3 28–37/48. Every depth-10 cave is the wall the Sanctum moved,
  and the Prior is tuned.
- No single knob in its range moves that floor: PackMul 0.7, WakeRange
  8, DmgQuad 0.01, ArmorCap 0.7, HPPerLvl 8, HpQuad 0.03, ShotMul 0.6,
  the abyss2 kit and 3000 gold all leave abyss1 at 1–4 of 48. Eight
  imps at 60% damage are still 80 a turn. The lever is structural: how
  many ranged packs an open cave converges on the hero.
- Nor does corner play: a bot that steps out of the shooters' sight and
  holds there kills six on one belt where it died in twenty turns, and
  still dies 44/48 on the floor and every time in the eval.
- Gambling with a full purse buys nothing, again: four rerolls and four
  gambles a trip spent 1000 g a death (sure) for gear on arrival flat
  to worse at every checkpoint.
- The caster's grotto wall is mana per trip: 27 of its 96 deaths are a
  lone champion Crystal Golem meleed with an empty pool and no gold
  (trips 38 a run). ManaPotion 0.5 puts its Oracle rate in band (0.27
  to 0.42, sure) for +260 surplus a level; the gold band takes the
  score back, as every progress lever has since 2026-09-30.
- The crypt bands do not move without the grotto paying: the damage
  reshape that cut crypt4 packLive 19 to 16 cost the fighter 1.4
  levels and 53 kills (sure). The crypt is a design question, not a
  knob.

## Report

Write `balance/REPORT.md`: problems found, bot changes, every kept change
with its reason and its numbers, simulations run, how the key metrics
moved (baseline → final, fighter vs caster), regressions reverted, remaining
concerns, the recommended configuration (`balance/best.json` as a
`DefaultRules` diff), and the code changes for review (`git diff --stat`).
Do not commit; the human applies the pick.
