# Updates

## 2026-10-09

- **Conversations, the Morrowind way.** Walk into anyone in Emberhold
  and the talk opens in a window: what was said on the left, topics on
  the right. Highlighted words in a line are topics; hear one and you
  can ask anyone about it. Hadrik and Mirela open on Barter. ↑↓ and
  enter, or click a topic or a highlighted word.

## 2026-10-08

- **Seven languages.** English, Deutsch, Français, Italiano, Русский,
  العربية and 简体中文. Choose one with `-lang de` or your `LANG`, or
  over SSH with `ssh -t medv.io -p 2222 de`. You can also change it on
  the title screen with ← → or a click. Arabic reads right to left with
  joined letters, and Chinese is drawn two cells wide. An LLM translates
  from one shared glossary; see [`docs/translating.md`](docs/translating.md).
- **Bigger Ashen Fields and Blackmarsh.**

## 2026-10-05

- The Last Wanderer no longer opens a portal of his own, and the ending
  choice is gone for now. Bug fixes from a review of the final fight.

## 2026-10-04

- `-area` picks where to start, and `-lvl`, `-level` and `-build` start
  a geared hero deep in the world.

## 2026-10-03

- **The Last Wanderer**, the final boss, waits at the bottom of the
  Burning Abyss. Emberhold goes dark while he is in it.
- **The Stolen Ember.** The game's text is rewritten around one story,
  told in [`docs/lore.md`](docs/lore.md).
- Talk text wraps to the box it is drawn in.

## 2026-10-01

- **The potion belt** has slots; click a slot to drink. The life and
  mana bars show what a potion will still restore.
- Overlays fit the screen, and lists hint at rows hidden below.
- Monsters wake from closer (WakeRange 16 → 10), and the portal opens
  beside the hero.

## 2026-09-30

- **Play over SSH:** `ssh medv.io -p 2222`, or host your own with Docker.
  Idle players are dropped, and the server limits sessions and
  connection rates.
- **Self-play balancing.** A scripted fighter and caster play the game
  to the end; watch one with `-bot fighter`.
- The numbers were rebalanced from those runs: armor, damage, items,
  shops by depth, gambling and rerolls. The town fountain restores life
  only. Grotto spiders come in packs of 2–4.
- shift+tab cycles targets backwards, and a click selects a target.

## 2026-09-29

- **First release.** A Diablo-like roguelike for the terminal, where
  torches, campfires, crystals, lava and spells cast real colored light.
- Mouse hover shows what is under the cursor in the status line.
- A potion belt with heal-over-time. Life no longer regenerates by
  itself, and nova freezes monsters.
