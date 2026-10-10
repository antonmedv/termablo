# Updates

<!-- When adding an entry, also update "Latest update" in README.md. -->

## 2026-10-10

- **The Cinder Sanctum.** A new area under Emberhold's own graveyard:
  the Ember Cult's chapel, every brazier lit again, and beneath it the
  Kindling, their digging toward the forge from below. Level 10 and 11,
  for after the Oracle. Its door is a stair among the graves, where
  Brother Aldous sits.
- **Two new side quests, from new givers.** Hadrik wants the coal the
  Cult took from his forge thirty years ago brought back: the first
  quest that is a thing to carry, not a head to bring. Pick it up and
  the whole floor wakes. Wear it on the way out if you dare. Aldous
  wants the Cinder Prior put out: Osric, the lamplighter who founded the
  Cult, remade by the fire. He throws fire, calls imps out of the
  braziers, and when hurt the fire takes him for good.
- **The Gravewardens' Barrow.** An earth ring south of the crypt road,
  two floors down to the Buried Captain, Edran's first guard. Ask Voss.
- **Two new faces in Emberhold.** Brother Aldous, the Ember Cult's last,
  sits among the graves and tells you what the town won't. Pell, the boy
  who followed the marsh lights, sits by the fountain and listens to
  the water. Villagers' rumors lead you to them.
- **The town argues about the Oracle.** Voss sends you to end her;
  Mirela begs you to listen first; Aldous calls her the only one who
  ever paid. After she falls, each of them, and Pell, takes it their
  own way.
- **Ask the one who knows.** Each part of the story has an owner who
  tells it best, and the rest of the town points you to them instead of
  repeating it.
- **See who you're talking to.** Every conversation opens with a
  glimpse of the person, in gray: Hadrik soot-black to the elbows,
  Mirela's crystal humming at her throat. The full picture the first
  time, a glance after, and what the story has changed in them since.

## 2026-10-09

- **Conversations, the Morrowind way.** Walk into anyone in Emberhold
  and the talk opens in a window: what was said on the left, topics on
  the right. Highlighted words in a line are topics; hear one and you
  can ask anyone about it. Hadrik and Mirela open on Barter. ↑↓ and
  enter, or click a topic or a highlighted word.
- **The town remembers.** Townsfolk greet you differently once they've
  met you, react once to each boss you kill, and say so when you ask
  them the same thing twice.
- **Quests are given, not known.** You start with none. Captain Voss
  asks for the Bone King when you meet him, and for the Drowned Oracle
  once he has paid for the king, or sooner if you ask him about her.
  The Oracle's death sets you on the Last Wanderer. Kill a boss before
  anyone asks and you are still paid.
- **A quest journal.** Press `J`, or click the quests in the side
  panel: every quest you've been given, what to do next, who gave it,
  where the boss waits, and a journal entry for each step along the way.
- **The world teaches you what to ask.** Entering the crypt, bleeding
  on an altar, reading a portal scroll or killing your first cultist
  gives you a topic to bring back to town. Voss's first briefing is
  shorter: the rest is yours to find.
- **Townsfolk see you.** Come back bleeding, broke, with an empty belt,
  or wearing something they know, and they say so. Villagers tell you
  rumors you haven't heard first, and their talk changes as the bosses
  fall.

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
