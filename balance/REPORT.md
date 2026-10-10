# Balance pass, 2026-10-10

Agent-driven pass over `termablo` at `07dcbcc` (the Cinder Sanctum merged;
working tree, uncommitted), starting from the 2026-10-01 pick. 16
evaluations (`balance/runs/109`–`124`), ledger rows #84–#99, about 3,000
full bot runs on seeds 1–96 (the work) and 1001–1096 (validation), plus
some 1,500 floor trials with the new Kindling instrument.

## Headline

The Sanctum did not create a wall, it moved one: before it, 18 of 21
post-Oracle runs died on abyss1; now 24 of 24 Kindling arrivals die there
and the Prior is never reached. A par hero dropped on the Kindling floor
dies 48/48 with him untouched and beats him 46/48 in a duel. The same hero
dies 46–47/48 on abyss1 and abyss2, 1/48 on the Sanctum chapel, 28–37/48
on grotto3. Every depth-10 cave is lethal, whatever the level number says:
seven or eight imps and cultists firing from range for 150–200 a turn
while hellspawn or wraiths close in, on a hero with 400 life. No single
knob within its range, no gear, no gold and no corner-fighting bot moves
it. That is the design question this pass hands back.

What the pass did move:

| | seeds 1–96 | seeds 1001–1096 |
|---|---|---|
| score (base → final bot → pick) | 13.35 → 12.60 → 14.25 | 13.91 → 13.26 → 13.84 |
| caster Oracle | 0.20 → 0.27 → 0.42 | 0.16 → 0.22 → 0.36 |
| caster deepest | 8.28 → 8.56 → 8.74 | 8.09 → 8.42 → 8.80 |
| fighter Oracle | 0.19 → 0.19 → 0.17 | 0.17 → 0.18 → 0.19 |
| gap | 0.08 → 0.08 → 0.25 | 0.06 → 0.06 → 0.18 |
| caster gold (band 0.8–1.5) | 3.82 → 3.75 → 4.33 | 3.76 → 3.51 → 4.75 |
| stuck of 192 | 0 → 0 → 0 | 1 → 1 → 2 |

The score is flat because the gains land where the goals do not want them,
as in both earlier passes: the caster's Oracle rate enters its band and the
gold band and the build gap take it back.

## Recommended configuration

`balance/best.json`; `DefaultRules` is untouched, the human applies it.

| knob | was | now | why |
|---|---|---|---|
| `ManaPotion` | 0.4 | 0.5 | a mana potion restores half the pool: the caster's grotto wall is mana per trip (27 of its 96 deaths were a lone champion Crystal Golem meleed with an empty pool and no gold, after 38 trips a run); Oracle 0.27 → 0.42 on seeds 1–96 and 0.22 → 0.36 on fresh seeds, both sure; trips −4 and −2.8; surplus +258 and +342 (sure) |
| `ShotMul` (new) | 4/5 hard-coded | 0.8 | a monster's shot as a fraction of its blow; 0.8 is the old value exactly. Measured at 0.6 and found not to be the lever; kept as the one knob on ranged damage |

ManaPotion 0.6 (#94) gives the same Oracle rate for twice the gold; 0.5 is
the smaller change. The fighter's Oracle rate wobbles 0.17–0.24 across
these runs (it drinks one mana potion a trip and casts a little), which is
what the gap goal measures here: noise, not a build drifting.

## Bot changes (kept)

- **A monster on the portal.** A probe over all 192 runs found 31 deaths
  with a hostile standing on the portal the bot was walking to, 20 of
  them with a scroll in hand: the walk attacked the monster on the cell
  until the hero died. `escape()` now reads a fresh scroll when the
  portal is blocked (or walled off) and one is in hand; with none, it
  fights through as before. Caster deepest +0.28 (seeds 1–96) and +0.32
  (fresh), both sure, clvl +0.5; fighter flat (#91, #98).

## Instruments added

- `make kindling [SEEDS=48 AT=abyss1 KIT=abyss2 GOLD=3000 RULES=..]`
  (`kindling_test.go`): a par hero as the bot arrives at the Kindling
  (lvl 22, the grotto3 kit, belt full, two scrolls), on the floor as
  generated and in a duel with the Prior; `AT` puts the same hero on any
  floor (the win is then the way down), `KIT` dresses it from another
  checkpoint, `GOLD` gives it a purse. 20 seconds for 192 trials. One run's
  log with the bot's state changes: `BOTKINDLINGLOG=fighter:3:false`.

## Problems found

1. **The depth-10 caves.** Floor trials, 48 seeds each, fighter/caster
   survived: grotto3 11/20, sanctum1 47/48, sanctum2 0/0 (1/0 at 400 g),
   abyss1 1/2, abyss2 1/3. With the abyss2 par kit: grotto3 29/28, abyss1
   1/1, sanctum2 0/0. With 3000 g: abyss1 1/2, sanctum2 0/0. Single knobs
   on abyss1 (fighter/caster of 48): PackMul 0.7 1/3, WakeRange 8 2/2,
   DmgQuad 0.01 1/4, ArmorCap 0.7 1/2, HPPerLvl 8 4/2, HpQuad 0.03 1/4,
   ShotMul 0.6 2/2. The kill is structural: an open cave with 25–27 packs
   of which 40–47% are ranged (imps in packs of 2–4, cultists), all of
   them converging and firing from where the melee cannot reach. The
   Sanctum chapel, rooms and corridors with 26 packs at lvl 10, is a
   walk: layout, not level.
2. **The Prior is tuned.** Duel: fighter 46/48 on 3.2 potions, caster
   46/48 on 3.0, 130 turns, his burning phase fires in 47–48 of 48. He
   does not need numbers; his floor does.
3. **The caster's grotto wall is mana per trip** (above). ManaPotion is
   the lever; BoltCostLvl 0.4 is half the effect (#88).
4. **The crypt bands cannot be reached by a global shape.** DmgLin 0.34 /
   DmgQuad 0.01 (+10% damage at the crypt, +6% at the grotto, 0 at lvl 11)
   cut crypt4 packLive 19.4 → 16.3 and cost the fighter 1.4 levels and 53
   kills (sure, #85). HpLin 0.42 / HpQuad 0.035 did not move the crypt at
   all (#86). The crypt is 3.9 of the score and a design question: either
   the crypt's own monsters or its level numbers, or the bands.
5. **Gold, again.** The fighter nets 7.8 gambles a level against a band
   of 0.8–1.5 and holds 2,200 g at death; the caster 3.8. Spending it
   (four rerolls and four gambles a trip) took 1,000 g off the fighter's
   death purse (sure) and bought gear on arrival that was flat to worse
   at every checkpoint (#92): a gamble at depth rarely beats gear that is
   already the best of a level's drops. The surplus metric counts income
   net of consumables, so no amount of spending moves it; only income
   cuts do, and those cost the caster its trips.
6. **A metric artifact.** `abyssDeath` takes the median death depth of
   runs that reached depth 10 or more; Sanctum deaths (depth 10–11) now
   count, so the 11–13 band reads as met (11) where it read 10 before the
   Sanctum. The goal meant the Abyss.

## Regressions found and reverted

| tried | effect | why reverted |
|---|---|---|
| DmgLin 0.34, DmgQuad 0.01 (#85) | crypt4 packLive −3 | fighter clvl −1.4, kills −53, caster deepest −0.41 (sure); Oracle 0.19 → 0.12 |
| HpLin 0.42, HpQuad 0.035 (#86) | caster Oracle +0.06 | crypt bands unmoved; score −1.24 is noise (−2.49…+0.29) |
| ChampBase 12 (#87) | — | caster Oracle 0.20 → 0.16, fighter kills.crypt4 −0.45 (sure) |
| ManaPotion 0.6 (#88, #94) | caster Oracle 0.41 | same as 0.5 for +524 surplus instead of +258 |
| BoltCostLvl 0.4 (#89) | caster trips −7 (sure) | Oracle +0.05 not sure: saves trips, does not win fights |
| StrDiv 40 (#90) | potions −0.7 a level (sure) | nothing else: the fighter's grotto deaths are pack fights |
| bot: shop until spent (#92) | goldHeld −1032 (sure) | gear flat to worse, caster deepest −0.31 |
| ShotMul 0.6 (#95) | — | floors unmoved, eval noise |
| bot: cover and hold vs shooters (#96) | corner play in the trace | abyss1 44/48 dead, Kindling 48/48, eval noise, one stuck |

## Simulations run

16 evaluations of 192 runs each (96 seeds × 2 builds), 47–78 s each: 11
on seeds 1–96, 3 on seeds 1001–1096 (the control from a clean worktree at
07dcbcc, the bot fix, the pick), one scratch run to show the ShotMul knob
at 0.8 reproduces the old code bit for bit, one probe over 192 deaths.
About 30 floor trials of 96–192 runs. Eight bot traces read end to end.

## Remaining concerns

- **The depth-10 caves** are the game's wall and nothing short of a
  structural change reaches them: fewer ranged packs in the abyss and
  kindling tables, smaller imp packs, fewer packs in deep caves, or a
  layout with corridors where the chapel has them. Each is content, not a
  knob; the Kindling trial measures any of them in 20 seconds (`AT=abyss1`
  for the abyss). Until one lands, the Prior quest is unreachable for the
  bot and the Abyss band (deaths at 11–13) is met only by the artifact.
- **The gold band** has now absorbed every progress lever in three
  passes. Either re-band it (2–4 would hold the current game) or change
  the metric to what piles up unspent (gold held at death per level).
- **The gap goal** at 96 seeds is dominated by the fighter's Oracle rate
  wobbling ±0.05; the pick reads 0.18–0.25 on it.
- **The crypt bands** (3.9 of the score) are a design question; see
  problem 4.
- **Stuck residue**: 1–2 of 192 on fresh seeds, all the known class (a
  broke caster with no mana wandering an explored level), none from the
  fix.

## Code changes for review

`git diff --stat -- '*.go' docs/ Makefile`: 5 files, +58/−3, plus
`kindling_test.go` (new), `balance/` (ledger rows #84–#99, runs 109–124,
rules files, `best.json`, `best-run.json`).

- `bot.go`: `escape()` re-reads when a hostile stands on the portal or
  walls it off.
- `rules.go`, `game.go`: `ShotMul` knob (combat, 0.4–1, default 0.8),
  read by `monsterShoot`.
- `kindling_test.go`, `Makefile`: the Kindling trial and `make kindling`.
- `docs/balancing.md`: the instrument row and this pass's lessons.

`make check` passes (fmt, lint, deadcode, tests). Not committed.
