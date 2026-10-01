# Balance pass, 2026-09-30

Agent-driven pass over `termablo` at `79571fb` (working tree, uncommitted).
56 evaluations, about 9,000 full bot runs (two builds, 20,000-turn cap),
plus sampler duels and reference tables per evaluation. Every evaluation
is in `balance/runs/`, every decision in `balance/LEDGER.md`
(`balance/ledger.jsonl`), the recommended rules in `balance/best.json`,
and the final human-readable table in `balance/report-final.txt`.

## Recommended configuration

Applied to `DefaultRules` in `rules.go`. Validated on fresh seeds
1001–1096 against the defaults on the same seeds and the same code:
score 19.62 → 14.52 (paired bootstrap −5.09, 95% −7.62…−1.44). Lower is
better; 0 would be every goal in band.

| knob | was | now | why |
|---|---|---|---|
| `ManaPerEne` (new) | 1.2 | 1.7 | the caster's kills per mana pool fell from 3 at crypt1 to 1.7 at grotto1 and it spent 18–34 town trips a run on potions; Energy-scaled mana reaches the caster only |
| `QuestGoldPerLvl` | 250 | 100 | income was 35–40% sells, 20–25% quest, 30% drops; the cut halves the gold surplus with no measurable cost to progress |
| `GoldPerLvl` | 3 | 2 | same |
| `StrDiv` | 60 | 45 | the fighter's only build-specific lever that measured: +15% melee at crypt4, fighter Bone King rate 0.67 → 0.72, build gap 0.15 → 0.09 on seeds 1–96 |
| `PotionHeal` | 0.5 | 0.65 | median potions left at death was 0 for both builds; caster deepest +0.29 (sure), fewer trips, fighter unchanged |

Plus one monster-table change: Grotto Spider pack 3–5 → 2–4, speed
130 → 120 (`monsters.go`). The typical grotto pack dealt 5.2 attacks a
turn against 2–3.5 in every other area.

Fresh-seed validation, paired by seed (final − defaults):

| | fighter | caster |
|---|---|---|
| Bone King slain | 67% → 72% | 79% → 89% |
| Oracle slain | 0 → 1 of 96 | 0 → 1 of 96 |
| reached grotto3 | 0 → 4 | 2 → 7 |
| median deepest depth | +0.57 (sure) | +0.62 (sure) |
| median character level | +1.06 (sure) | +1.04 (sure) |
| town trips per run | 7.8 → 7.2 | 21.3 → 19.0 (sure) |
| gold surplus per level, in gambles | 5.8 → 5.1 | 4.0 → 3.2 |
| stuck runs | 0 → 0 | 0 → 0 |

## Problems found

1. **The grotto wall.** Every run died in grotto1–2; the Oracle was
   never reached. The 1v1 survival band sat in range at grotto1 while
   45 of 96 runs died there: packs decide, and the typical grotto pack
   was twice as dangerous as anywhere else. A pack-aware survival
   measure (`packLive`, `fight`) now scores this; the spider change
   put grotto1 in band. Deaths then moved to champion packs (golem,
   drowned, spider champions: 25 of 56 grotto deaths) in open caves.
2. **The caster runs dry.** Bolt cost grows with level, the pool with
   Energy and level, monster life quadratically: kills per pool fell
   from 3 to 1.7, so the caster shuttled to town for potions and died
   walking home in melee. Mana had no knobs; four were added.
3. **Attrition deaths.** Median potions left at death: 0, with 3–4
   enemies adjacent and 10–30 awake. The bot called a portal only at
   35% life with an empty belt, too late for a two-turn exit, and
   counted an undrained potion pool as life.
4. **Gold.** Surplus after consumables was 6.4 (fighter) and 4.1
   (caster) gambles per level against a 0.8–1.5 target, and 35–40% of
   income came from selling drops. See "remaining concerns": the
   surplus is the fighter's gear pipeline.
5. **Crypt too safe against packs** (packLive 16–19 vs 8–12) while the
   grotto was too deadly: a cliff between depth 5 and 7 rather than a
   curve.

## Bot changes (needed, and measured before any rules moved)

- `losing()`: leave by portal at <50% life with an empty belt or <40%
  with one potion, while a threat is in view. A first version that also
  fled at <50% with three adjacent while potions remained was worse
  (fighter Bone King 62% → 35%): a potion beats two exposed turns.
- `hp()` counts only the next turn's potion drain, not the whole pool
  (the bot read 17 real life as "38%").
- `retreat()` backs into a nook with fewer open sides; caves have no
  corridors.
- Drink only when the pool has room: the game refuses a drink past
  full life without spending a turn and the bot looped on it (2 stuck
  runs).
- Progress accounting: gold and pack changes and the walk to known
  stairs count as progress for the 300-turn stall rule.
- Auto-explore (game code, the `o` key too): a cell the hero stood on
  whose unseen neighbours stayed unseen is spent and never targeted
  again; the bot's own frontier walk keeps its goal until reached. Two
  earlier attempts at the same loop (explored after 40 stale turns) were
  reverted after 34–39 stuck runs each.
- Instruments: killer rank, adjacent and awake enemies and potions left
  at death; bolts and novas cast; a text trace of one run's end
  (`BOTTRACE=fighter:7:60 go test -run BotTrace`).

Bot fixes were kept on correctness, not score: none moved the score
beyond noise except the escape rule (fighter Bone King 62% → 71%).

## Regressions found and reverted

| tried | effect | why reverted |
|---|---|---|
| ManaPerLvl 2 → 4 | score −3.6, caster grotto3 1 → 5 | fighter Bone King 0.68 → 0.55 (sure): a shared stat is the wrong lever for a caster problem |
| BoltEnePow 1.2, BoltMul 0.75 | caster king 0.92, oracle 3 | a buff from crypt4 on, gap 0.33 |
| weaker champions (Hp 1.8, Dmg 1.2) | caster grotto1 69 → 82 | fighter unchanged, gap 0.33: champions are what the fighter farms |
| fewer champions (PerDepth 2, 1.5) | nothing | same |
| DmgQuad/DmgLin, HpQuad/HpLin reshaping | nothing / fighter −0.08 | the wall is not a 5–10% number |
| SellDiv 24, 16, 14; gamble and reroll prices ×1.7 | gold bands met (2.4/1.3) | fighter clvl −0.5 to −0.9 and king −0.09 to −0.15, all sure: sells and gambles are the gear budget |
| MonArmorK 200, ToHitPerLvl 1 | nothing / fighter −0.14 | fighter-only levers that did not measure |
| fighter bolts only awake targets | nothing | simpler bot stays |

## Remaining concerns

- **Oracle 1% vs 30–45%.** Grotto2–3 is still a wall for both builds.
  Nothing within ±10% of any monster number moved it; what moved the
  caster (mana, potions, weaker champions) left the fighter behind.
  Likely needs design, not numbers: all-champion packs deep (a leader
  and normals instead), cave chokepoints, or a defensive tool for melee.
- **Gold band vs the fighter.** Every change that met the 0.8–1.5
  gambles-a-level band starved the fighter of gear. Either the band is
  too strict for these rules (2–4 would fit the data) or gamble quality
  should rise with price.
- **Crypt vs packs** stays 16–19 (band 8–12) because every attempt to
  raise early monster stats cost the fighter; the band may be too
  strict for a game with champion packs.
- **Caster ahead early**: 2 bolts per skeleton at crypt2 (band 3–5),
  Bone King 89% vs 72%; gap 0.17 (band ≤ 0.15). Caster buffs were
  reverted for this reason; the caster's edge is range and Nova.
- ~~Stuck runs ~1%~~: found in review to be the bot's "no way there"
  mark made permanent through `Level.Spent`; fixed, 0 stuck of 384
  runs on both seed sets (ledger #41, #42).
- **Bot ceiling**: no boss priority in target choice, no kiting, no
  chokepoint fighting; `equips` (6 a level) is bot taste. The abyss is
  measured only by the reference table since no run arrives.
- **Exploits looked for, not found**: reroll and gamble sinks stay
  small, gold held at death is small, town trips are bounded by scroll
  prices, no farming loop (kills per level fall with depth). Deliberate
  farming needs a policy variant the bot does not have.

## Code changes for review

`git diff --stat`: 18 files changed, ~670 insertions, ~240 deletions,
plus new `eval_test.go`, `trace_test.go`, `internal/balance/`,
`cmd/balance/`.

- `rules.go`: knobs carry a description, a step and a whole-number flag;
  six new knobs (`ManaPerEne`, `ManaPerLvl`, `BoltCostLvl`, `ManaPotion`,
  `SellDiv`); `rulesFromFile` JSON overlay, `knobValues`, `knobDiff`;
  the five defaults above.
- `player.go`, `game.go`: mana pool, bolt cost, mana potion and sell
  price read the knobs; `Stats.Death`, spell counters, the log counter
  for the trace, auto-explore spent cells.
- `level.go`: `Spent`. `monsters.go`: the spider. `stats.go`: death and
  spell counters. `items.go`, `hover.go`, `ui.go`: `sellPrice` and
  `Lines` take rules.
- `bot.go`: the changes above.
- `internal/balance`: rows, metrics, goals, score, bootstrap, paired
  comparison. `cmd/balance`: `show`, `compare`, `seeds`, `log`, `ls`.
- Tests: `eval_test.go` (`make eval`, `make knobs`), `trace_test.go`,
  `bot_test.go` and `ref_test.go` split into compute and print,
  fixtures take rules, `rules_test.go` covers the registry and overlay,
  `scenario_test.go` reads the quest knob.
- `Makefile`: `report` takes `SEEDS`/`FIRST`/`RULES`; `eval`, `knobs`;
  deadcode over `./...`.
- Local only (excluded from git): `balance/`, `.claude/skills/balance/`
  (the runbook for the next pass).

`make check` passes (fmt, lint, deadcode, tests). Committed as five
commits a9b4811..914022d plus two review follow-ups (`fix(bot): no
permanent frontier marks...`, `test: review follow-ups...`); the review's
14 findings are all addressed.
