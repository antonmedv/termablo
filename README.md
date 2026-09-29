# Termablo

A Diablo-like roguelike for the terminal, written in Go with Bubble Tea.
Lighting is ray-cast, colored and animated, and drawn on a black background.

```
go run .            # random world
go run . -seed 42   # the same world every time
```

Use a truecolor terminal that is at least 80x24. Bigger is better.

## World
- **Emberhold**: a walled town with lanterns, a forge, the alchemist's crystal and a graveyard.
  - Hadrik sells arms and armor. Mirela heals you for free and sells potions, portal scrolls and jewelry. Captain Voss gives the quests.
- **Ashen Fields**: moonlit grassland with Fallen campfires, ruins, a graveyard, and the way down to the crypt.
- **Crypt of the Fallen 1–4**: rooms and corridors lit by braziers. The **Bone King** waits at the bottom.
- **Blackmarsh**: swamp water, standing stones, will-o'-wisps and a blood altar.
- **Sunken Grotto 1–3**: caves lit by blue crystals. The **Drowned Oracle** waits at the bottom.
- **The Burning Abyss 1…∞**: lava caverns that keep going down and keep getting harder.

## Balance
- Life and mana do not regenerate on their own. They come back from potions, shrines, Mirela and regeneration affixes on gear.
- Potions restore over a few turns rather than at once. The belt holds 5 of each kind, and Mirela charges more as you level.
- Spell damage grows with Energy only. Firebolt reaches 10 steps; bats, wolves, wisps, imps and spiders can dodge it.
- Frost Nova freezes for about 2 turns, and the target can't be frozen again for 6 turns. It never freezes uniques or bosses and freezes champions for only 1 turn.
- Merchants pay 1/12 of an item's worth, and next to nothing for plain gear. Plain monsters rarely drop items.

## Lighting
- Every light casts its own rays and is blocked by walls. Lights add up by color: warm torches and braziers, cool crystals, lava, portals and glowing monsters.
- Light falls off softly with distance and is tone-mapped. Light circles are corrected for the cell aspect ratio so they look round.
- You see a cell only if it is in your line of sight **and** lit. A monster standing in darkness stays invisible until light reaches it. A firebolt lights up the corridors it flies through.
- Rare and unique drops glow on the ground.

## Keys
| key | action |
|---|---|
| arrows / hjkl yubn / numpad | move and attack |
| `f` / `r` | Firebolt / Frost Nova |
| `tab` | cycle target |
| `q` / `w` | healing / mana potion |
| `t` | town portal |
| `g` | pick up |
| `o` | auto-explore |
| `i` `c` `m` `?` | inventory, character, map, help |
| `Q` | quit |

## Code
- `light.go`: ray casting for sight and light, plus flicker.
- `game.go`: turns, combat, AI and town.
- `gen.go`: map generators.
- `items.go`: bases, affixes and uniques.
- `ui.go`: renderer.
- `screen.go`: cell buffer and ANSI serializer.

## Tests
```
make check     # gofmt, golangci-lint, deadcode, tests — run before committing
make test      # go test ./... (~8s); make test-short runs fewer seeds
make fuzz      # 60s of random key input vs. game invariants
make report    # bot balance report over 12 seeds
make bench     # frame/turn benchmarks into bench.txt (compare with benchstat)
make shots     # render screenshots into shots/ (needs rsvg-convert)
```
Tools: `brew install golangci-lint`, `go install golang.org/x/tools/cmd/deadcode@latest`. Lint rules live in `.golangci.yml`.
- `testutil_test.go`: `newTestGame` builds a game from an ASCII map; `eachLevel` runs a check on every generated level over many seeds.
- `gen_test.go`, `items_test.go`, `light_test.go`: invariants for levels, loot and lighting.
- `combat_test.go`, `explore_test.go`, `scenario_test.go`: gameplay scenarios (shops, portals, stairs, death, level-up, gear), mostly driven through the real key handler.
- `bot_test.go`: a scripted player runs 12 seeds and logs a balance report (`go test -v -run BotBalance`, `BOTSEEDS=n` for more).
- `sim_test.go`, `tea_test.go`: smoke runs and benchmarks.
