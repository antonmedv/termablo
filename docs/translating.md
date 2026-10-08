# Translating Termablo

Termablo speaks English, German, French, Italian, Russian, Arabic and
Simplified Chinese. English is the source: every line is written there
first, and a translation that lacks a line shows the English one. The
translations are made and kept up to date by an LLM working from one
shared glossary, through the tool in `cmd/i18n`. This is the runbook.

## Playing in a language

```sh
go run . -lang de                 # or LANG=de_DE.UTF-8 go run .
ssh -t medv.io -p 2222 ru         # over SSH: the language as the command,
                                  # or the LANG your ssh client sends
```

The title screen lists every language by its own name: ← → (or a
click) picks one, the screen switching to it as you go; enter begins. `ch`,
`cn` and `zh_CN` are accepted for Chinese.

## The files

```
locales/
  en.maml          the source: every key, with comments for translators
  de.maml ...      one per language, written by `i18n merge`
  glossary.maml    the terms every translation shares, and each language's style
  prompts/         the LLM prompts: prompt.md translates, review.md reviews
  lock/de.lock     which English each translated line was made from
internal/i18n/     catalogs, plurals, genders, the checks; RTL and wide text
cmd/i18n/          the tool
i18n.go            the game's side: naming items, monsters and places
```

The catalogs are [MAML](https://maml.dev). Nested objects are
namespaces; a key is the dotted path, `msg.pickup_gold`.

## What an entry can be

- **A string** with `{placeholders}`: `"You pick up {n} gold."`
- **Plural forms**, picked by the count `{n}` with the CLDR rules of
  the language (`internal/i18n/lang.go`):
  `{ one: "...", few: "...", many: "..." }`. English needs one form where
  Russian needs three and Arabic six; the translation adds them.
- **Gender forms**, picked by the gender of a noun argument:
  `{ m: "Gezackter", f: "Gezackte", n: "Gezacktes", pl: "Gezackte" }`.
  A missing form falls back to its first letter (`mp` → `m`), then to
  `other`.
- **A noun**: `{ text: "Dolch", gender: "m" }`. Item bases, monsters,
  rare-name nouns and unique names are nouns, so the words around them
  can agree.

Variants nest. A comment `# max: N` above a key (or a namespace) caps
its width at N terminal cells; a CJK character takes two.

How names are built: a magic item is `item.magic` filled with
`prefix.*`, `base.*` and `suffix.*`, each affix chosen in the base's
gender; a rare item is `item.rare` from `rare.first.*` agreeing with
`rare.second.*`. A monster in a message is `{who}`: `ref.the` or
`ref.a` around the monster's noun, in the monster's gender, or a named
monster's name alone. Translations keep `{who}` as the subject, so no
grammatical case beyond the nominative is ever needed.

## In the code

The game logic keeps its English names: tables, Stats, the bot, the
balance reports. Only what the player reads is translated, through the
session's catalog `g.L` (every SSH player has their own):

```go
g.say(colGold, "msg.pickup_gold", "n", it.Amount)         // to the log
g.L.T("ui.panel.gold", "n", p.Gold)                       // any text
itemNoun(g.L, it), monsterNoun(g.L, m), theRef(g.L, m)    // names
```

Arguments come in name, value pairs. Rows of the game's tables (bases,
affixes, monsters, tiles, mods) are keyed by the slug of their English
name, `Executioner's Axe` → `base.executioners_axe`. The tests hold
the catalog to the code:

- `TestCatalogCoversData`: every table row has its English line.
- `TestCodeKeysExist`: every key written in the code is in `en.maml`.
- `TestEnglishNames`: names built from parts match the English tables.
- `TestLocalesValid`: no shipped catalog has errors.
- `TestDrawEveryLanguage`: every screen draws in every language, no key
  showing through.

Text reaches the screen through `Screen.Text`, which measures wide
characters as two cells, and for Arabic joins the letters and lays each
line out right to left (`internal/i18n/bidi.go`): most terminals do
neither. `i18n.Width`, `fit` and `i18n.Wrap` replace rune counting
wherever text is measured.

## Changing English text

1. Edit `locales/en.maml` (and the code, for a new key). Comment the
   key where a translator needs context: who speaks, what fills a
   placeholder, how much room there is.
2. `go test ./...`
3. `go run ./cmd/i18n status` now shows the key missing or stale in
   every translation; the English still shows meanwhile.
4. Translate the change for each language, below.

## Translating with an LLM

```sh
go run ./cmd/i18n prompt -lang de > /tmp/de.md     # what is missing or stale
# give /tmp/de.md to the LLM; save its answer, a MAML object, to /tmp/de.maml
go run ./cmd/i18n merge -lang de /tmp/de.maml      # entries with errors are rejected
go run ./cmd/i18n check -lang de                   # 0 errors; read the warnings
go run ./cmd/i18n review -lang de > /tmp/review.md # a second pass, by a second LLM
go run ./cmd/i18n merge -lang de /tmp/fixes.maml
```

The prompt carries everything a translator needs and nothing else: the
game and its tone, the format, the language's plural categories and
genders, the style notes and glossary from `glossary.maml`, up to 60
lines already translated for consistency, and the entries to do with
their comments. `-only msg.` narrows it to one namespace, `-n 100` caps
its size, `-all` retranslates everything.

`merge` checks the answer as if it were the whole catalog, keeps the
good entries, writes the catalog in source order and records each
entry's English in `lock/<lang>.lock`. When that English later changes,
the entry turns stale: still shown, listed by `check`, and offered again
by `prompt` with the old translation beside the new English.

In Claude Code, `/translate de` runs this loop.

## What check checks

Errors (the translation is wrong and `merge` rejects it): a key the
source does not have, a placeholder lost or invented, a variant that is
neither a plural category nor a gender of the language, a gender the
language does not have, an empty text, an English line over its
`# max`.

Warnings: a stale or missing entry, a plural form missing, a noun
without a gender, a line over its `# max`, a glossary term the English
uses and the translation does not.

## The glossary

`locales/glossary.maml` is the one place a term is decided: the names
of people, places and bosses, the game's words (Ember, the Dark,
Firebolt, Life, Champion...), each with a note on what it is and its
form in every language. Russian follows `docs/lore.ru.md`. List the
inflected forms where a language declines, so the check recognizes them:
`ru: ["Уголь", "Угля", "Углю", "Углём", "Угле"]`.

Changing a term: edit the glossary, then `check` lists every line that
still uses the old one; `prompt -all -only <namespace>` or a review pass
brings them in line.

## Adding a language

1. A row in `i18n.Langs` (`internal/i18n/lang.go`): code, names,
   direction, plural categories, genders; and its plural rule in
   `PluralCategory`, from the
   [CLDR tables](https://www.unicode.org/cldr/charts/latest/supplemental/language_plural_rules.html).
2. The language's style in `glossary.maml`, and its form of every term.
3. `go run ./cmd/i18n prompt -lang xx`, and the loop above until `check`
   is clean.
4. `go test ./...` draws every screen in it.

## Known limits

- The panels keep their left-to-right layout in Arabic; the text in
  them, the log and the dialogs read right to left.
- Terminals that run their own bidi algorithm (mlterm, Konsole with
  bidi on) reverse Arabic a second time.
- Chinese needs a font with CJK glyphs, and a terminal that draws them
  two cells wide, as all common ones do.
- The `-bot` overlay and its trace stay in English: they are tools for
  balancing, not part of the game.
