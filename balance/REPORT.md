# Balance pass, 2026-10-01

Agent-driven pass over `termablo` at `db71fc7` (working tree, uncommitted),
starting from the 2026-09-30 pick. 36 evaluations (`balance/runs/057`–`092`),
ledger rows #40–#70, about 7,000 full bot runs on two seed sets (1–96 for
the work, 2001–2096 for validation), plus the sampler and the reference
table per evaluation. The human-readable table of the recommended
configuration is `balance/report-final.txt`.

## Headline

The grotto wall of the last pass was mostly a bot trap. Every traced grotto
death read the same: the bot had backed into a cave nook with three open
sides, three attackers filled it, the belt emptied, and the portal it then
read opened on the far side of them. Three adjacent with no potions and a
portal two cells away was the modal death of both builds. Fixing the
escape, and then the two-goal loops the fix exposed, is most of this pass:

| seeds 2001–2096, paired | old bot (#58) | this bot (#68) |
|---|---|---|
| score | 16.08 | 13.71 (the first validated step, #59: −2.50, 95% −5.52…−0.34, sure) |
| fighter: deepest / clvl / Bone King | 5.79 / 11.2 / 66% | 6.68 / 12.6 / 86% |
| caster: deepest / clvl / Bone King | 6.56 / 12.5 / 81% | 7.38 / 13.9 / 96% |
| Oracle (fighter / caster) | 0 / 0 | 1% / 4% |
| reached grotto3 (fighter / caster) | 2 / 9 | 8 / 16 |
| stuck runs of 192 | 1 | 0 |

Then one rule moved the wall itself: `WakeRange` 16 → 10, a new knob for
how close a monster in view must be to notice the hero (it was a hard 16
in `monsterTurn`). In an open cave every pack within 16 cells and a line
of sight joined a fight; deaths came with 10–40 awake.

## Recommended configuration

Applied to `DefaultRules` in `rules.go` and to `balance/best.json`. By
score alone the best-known rules were the defaults with the bot fixes
(#67, 13.19); the pick goes one step past that, on the numbers below, as
a design choice the score cannot settle:

| knob | was | now | why |
|---|---|---|---|
| `WakeRange` (new) | 16 | 10 | monsters notice the hero at 10 cells, not 16; a fight in the grotto no longer pulls every pack in sight |

With the final bot, paired by seed:

| | seeds 1–96 (#69 vs #67) | seeds 2001–2096 (#70 vs #68) |
|---|---|---|
| score | 13.19 → 13.84 (noise) | 13.71 → 13.58 (noise) |
| fighter deepest / clvl | +0.73 / +1.25 (sure) | +0.32 / +0.58 |
| fighter Bone King | 0.78 → 0.92 (sure) | 0.86 → 0.89 |
| caster deepest / Bone King | +0.22 / 0.95 | +0.15 / 0.96 |
| Oracle (fighter / caster) | 3% / 4% | 2% / 5% |
| reached grotto3 | 9/19 → 16/15 | 8/16 → 15/20 |
| build gap | 0.16 → 0.03 | 0.07 → 0.07 |
| gold surplus/level, fighter | 4.66 → 5.25 gambles | 4.63 → 4.97 |

The score stays flat because the gain lands where the goals do not want
it: the fighter's Bone King rate crosses the band's ceiling (0.85) and
income rises with depth. 12 and 8 were tried (#55, #56): 12 is half the
effect, 8 the same as 10. The next pass should take WakeRange 10 as its
base and make the crypt harder, which the crypt pack bands (packLive
16–19 against 8–12) and the king band both ask for.

## Problems found

1. **The nook trap.** `retreat()` backed into any cell with three or fewer
   open sides. In a cave that is a dead-end nook; three monsters fill it
   and the portal, placed nearest to the cell east of the hero, lands
   behind them. Fixed in the bot (corridors only) and in `readPortal`
   (nearest free cell to the hero).
2. **Escapes that walked.** With a portal open up to 8 cells away the bot
   walked to it under attack, 14 turns in one trace, with seven scrolls in
   the pack. It now reads a fresh scroll unless the portal is within two
   steps along known ground.
3. **Deaths now come broke.** With the escape working, 60–65 of 96 deaths
   per build end with 0 scrolls and about 30 g: at grotto1 the fighter
   kills 3–5 monsters a belt, gambles 330 g and rerolls for 120 g every
   trip, and is at 2 g after five trips. A two-refill reserve halves the
   broke deaths at no cost to gear on arrival.
4. **Two-goal loops.** Three stuck classes, all the same shape: a goal a
   step away from another goal's reach. A sleeping zombie 13 cells down
   a corridor showed from one cell and not the next, and the frontier
   route flipped between the way past it and a detour; an awake spider
   on an island showed from one cell, and the chase step took it out of
   view while the loot pulled back. Monsters seen are now remembered like
   the map, and a generic wobble guard leaves the enemies in view be
   after six turns of A, B, A, B.
5. **Pack convergence.** Deaths carried 10–40 awake monsters; the wake
   range was a hard-coded 16 with no knob.

## Bot changes (kept, each measured on seeds 1–96 and the fresh set)

- `retreat()` backs only into true corridors (≤ 2 open sides), never a
  three-sided nook (#42, with the portal placement: score 18.02 → 13.57,
  −4.44, 95% −7.96…−0.81, sure).
- `escape()` reads a fresh scroll unless the open portal is within two
  steps (#44): deaths with a scroll in hand 61 → 36 (fighter), 45 → 31
  (caster); 7 fighter seeds slightly worse, caster slightly better.
- `reserve()` keeps this trip's refill and the next one's before Hadrik
  gets anything (#53): broke deaths 35 → 25 and 51 → 35, gear on
  arrival unchanged.
- `remember()`: every monster in view is remembered where it stands (a
  sleeper until seen awake, dead or gone; one awake for 3 turns), and
  `pass()` routes around remembered cells from any cell (#65).
- `wobble()`: six turns between the same two cells leaves every enemy in
  view be for 40 turns; an ignored enemy that is adjacent is a threat
  all the same (#67). 0 stuck of 384 runs on both seed sets.
- Instrument: scrolls left at death (`Stats.Death.Scrolls`, row
  `ScrollsLeft`, metric `deathScrolls`).

## Regressions found and reverted

| tried | effect | why reverted |
|---|---|---|
| pre-emptive portal at belt ≤ 3, fight beside it, nook dropped (#41) | caster grotto3 2 → 9 | fires from the first fight (start kit 3 potions, 1 scroll): fighter king 0.74 → 0.61, surplus −150 (sure), 4 stuck |
| leave line rising with attackers (#43) | noise | fighter deepest −0.31, kills −17: trips cut short |
| sleeper walked through, not around (#61) | 0 stuck | caster deepest −0.43, Oracle 6% → 1%: engages packs it used to bypass |
| lost prey ignored 40 turns (#62) | — | caster clvl −0.65: it kites, so targets leave view every other turn |
| prey chased by memory (#66) | fixes seed 2044 | fighter king 0.82 → 0.67 and caster clvl −0.91 (sure), 3–5 stuck |
| PotionPricePerLvl 2 (#45) | nothing | surplus +69/+189 (sure) only |
| ChampHp 1.8 (#46) | fighter clvl +0.80, caster deepest +0.41 (sure) | caster king 0.97; adds nothing on top of WakeRange 10 (#57) |
| GoldPerLvl 3 (#47) | fighter deepest +0.35, clvl +0.64 (sure) | income: surplus +300 (sure), the gold band takes it all |
| HPPerLvl 6 (#48) | nothing | 9% more life at grotto1 buys nothing measurable |
| PackMul 0.8 (#49) | fighter deepest +0.49 | surely worse: caster clvl −0.85, kills and XP gone, crypt bands further out |
| MonToHitPerLvl 2, ChampDmg 1.2 (#50, #51) | fighter +0.05 king | noise |
| one champion leading normals, code (#52) | score 11.56 | all of it the gold band: income −3000/−4265 (sure), clvl −0.5/−1.1, deepest unchanged: champion packs are not the binding constraint |
| WakeRange 12, 8 (#55, #56) | half / same as 10 | 10 stays |

## Simulations run

36 evaluations of 192 runs each (96 seeds × 2 builds, 20,000-turn cap),
30 s each: 26 on seeds 1–96, 10 on seeds 2001–2096 (the control for the
old bot, each bot stage, and the pick). Twelve bot traces read end to end.

## Remaining concerns

- **Oracle 2–5% against 30–45%.** Deaths moved from grotto1 to grotto2–3
  but each grotto level still takes a third to a half of the arrivals,
  mostly champions, and the survivors arrive broke. Of 36 grotto3
  arrivals under WakeRange 10, 8 killed the Oracle: the boss itself is
  not the wall, the three cave levels are.
- **The gold band vs progress, again.** Income levers (GoldPerLvl, and
  WakeRange through more kills) move the fighter and the band takes it
  back; every death at the grotto is a broke one. Either the band is
  too strict (the previous report's 2–4 would fit) or the grotto needs
  to pay for its own trips.
- **The king band's ceiling.** Both builds now sit at or above 0.85
  (fighter 0.86–0.92, caster 0.93–0.96). The crypt is easy for the
  pack bands too (packLive 16–19 vs 8–12). Making the crypt harder is
  the lever that satisfies both and is untested with the new bot (the
  previous pass's HpLin/HpQuad reshaping was measured on the trap).
- **Caster ahead early** stays: 2 bolts per crypt2 skeleton (band 3–5),
  Bone King 0.94–0.96. Caster-only buffs (BoltCostLvl, ManaPotion) were
  not run for that reason; they are the obvious levers if the caster's
  21 trips a run are judged too many.
- **Bot ceiling.** No threat assessment: it engages every champion pack
  and returns through the portal into the same one until broke. No
  farming policy, no kiting. `equips` is taste.
- **Exploits looked for, not found**: the two-refill reserve cut gear
  and gamble spend without a gear loss on arrival, so the surplus was
  not buying power; gold held at death is ~30 g for the broke and
  700–1200 g for the rest; trips are bounded by scroll prices.

## Code changes for review

`git diff --stat -- '*.go' docs/`: 9 files, +179/−54; `balance/` adds the
ledger rows, 36 runs, the rules files and `report-final.txt`.

- `bot.go`: `retreat()` corridors only; `escape()`/`within()`;
  `reserve()` two refills; `remember()`, `sighting`, `known`, `held`,
  `pass()`; `wobble()`, `trail`; `threats()` adjacency exception.
- `game.go`: `readPortal` opens nearest the hero; `monsterTurn` reads
  `WakeRange`; `recordDeath` keeps scrolls.
- `rules.go`: `WakeRange` (combat, 8–20, whole number, default 16).
- `stats.go`, `bot_test.go`, `eval_test.go`, `internal/balance`:
  scrolls at death as a row and a metric.
- `rules_test.go`: `WakeRange` among the whole-number knobs.
- `docs/balancing.md`: lessons from this pass.

`make check` passes (fmt, lint, deadcode, tests). Committed on `balance`
and merged to `master` with the pick applied.
