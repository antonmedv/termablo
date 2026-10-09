You are translating Termablo into {{.Lang.Name}} ({{.Lang.Native}}).

Termablo is a dark-fantasy roguelike played in a terminal, in the tradition
of Diablo: a lone stranger, the Wanderer, is hired by Emberhold, the last
lit town, to descend through a crypt, a drowned grotto and a burning abyss.
Light is the core mechanic: you see only what light reaches. The tone is
spare, grave and a little weary, never jokey or modern. Sentences are short.
Quest-givers speak plainly; villagers speak in rumors.

Translate each entry below from English into {{.Lang.Name}}, as a game
localizer would: natural, idiomatic {{.Lang.Name}} that a native player of
dark-fantasy RPGs expects, not a word-for-word rendering.

## The format

Entries are MAML (like JSON: `key: value`, comments start with `#`,
`"""` opens a multi-line string). Keys are dotted paths in quotes; keep
every key exactly as given. A value is one of:

- a string: `"You pick up {n} gold."`
- plural forms, chosen by the count `{n}`. {{.Lang.Name}} uses these
  categories: {{join .Lang.Plurals ", "}}. Write them when a text
  with `{n}` needs them, e.g. `{ {{range $i, $c := .Lang.Plurals}}{{if $i}}, {{end}}{{$c}}: "..."{{end}} }`.
  English often has one form where you need several.
{{- if .Lang.Genders}}
- gender forms, chosen by the gender of the noun argument (an item base,
  a monster): `{ {{range $i, $g := .Lang.Genders}}{{if $i}}, {{end}}{{$g}}: "..."{{end}} }`.
  Use them for adjectives and articles that agree: a magic item's prefix
  agrees with its base, a rare name's first word with its second, an
  article with its monster. Only write the forms that differ; a missing
  form falls back to its first letter's (mp → m), then to `other`.
- a noun: `{ text: "...", gender: "..." }` with one of {{join .Lang.Genders ", "}}.
  Every noun must carry its gender: item bases (`base.*`), `gamble.*`,
  `rare.second.*`, monsters (`monster.*`), `unique_monster.first.*`,
  unique items (`unique.*.name`), and the potion and scroll names.
{{- else}}
- {{.Lang.Name}} has no grammatical gender: write nouns as plain strings
  even where the English is `{ text, gender }`.
{{- end}}
- Variants nest: gender forms may hold plural forms.

Placeholders like `{n}`, `{who}`, `{item}` are filled by the game. Keep
every placeholder of the English, never invent new ones, and move them
wherever {{.Lang.Name}} word order wants them. What they hold is in the
comments. `{who}` is always a full noun phrase in the nominative, article
included ("the Plague Rat", "Bloodmaw the Hungry"): build sentences with
{who} as the subject, so no other grammatical case is needed. `{item}` is
an item's name without an article.

Spoken lines may hold links, `[words](topic)`: the words show
highlighted, and hearing them teaches the hero a topic to ask about.
Translate the words, keep the `(topic)` exactly as it is, and keep every
link of the English, around whichever words name the thing in
{{.Lang.Name}}. Never add a link the English does not have.

Comments above an entry are context: who speaks, what fills a
placeholder, where the text appears. `# max: N` means the text must fit
in N terminal cells (each placeholder counted as two cells, a two-digit number, unless the max line says e.g. `n=1`); keep it
shorter, abbreviate if you must. {{if eq .Lang.Code "zh"}}A Chinese character
takes two cells.{{end}}

Key names on the keyboard (`g`, `tab`, `esc`, `ctrl+c`, `hjkl`, `?`) stay
as they are. Keep symbols like `·`, `—`, `▲`, `↑↓`, `←→` and the trailing
`g` that means gold in "{n}g".

## Style
{{if .Style}}
{{.Style}}
{{end}}
## Glossary

Use these translations for these terms, every time, inflected as the
sentence needs. "-" means keep the English.

| English | {{.Lang.Name}} | Notes |
|---|---|---|
{{- range .Terms}}
| {{.En}} | {{.Forms}} | {{.Note}} |
{{- end}}
{{if .Examples}}
## Already translated

For consistency, here is how some entries are translated already. Do not
repeat them in your answer.

```maml
{{range .Examples}}"{{.Key}}": {{.Current}}
{{end -}}
```
{{end}}
## Translate these

{{- range .Entries}}
{{range .Notes}}
# {{.}}{{end}}
{{- if .Max}}
# max: {{.Max}}{{end}}
{{- if eq .Status "stale"}}
# The English changed. The old translation was: {{.Current}}{{end}}
"{{.Key}}": {{.Source}}
{{- end}}

## Your answer

Answer with one MAML object and nothing else: `{` then one line per
entry, `"key": value` with the dotted key quoted exactly as given, then
`}`. No commentary, no code fence. Translate every entry.
