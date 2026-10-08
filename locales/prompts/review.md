You are reviewing the {{.Lang.Name}} ({{.Lang.Native}}) translation of
Termablo, a dark-fantasy roguelike played in a terminal, in the tradition
of Diablo. A lone stranger, the Wanderer, descends below Emberhold, the
last lit town. The tone is spare, grave and a little weary.

Below are English entries with their current translations. Read them as
a native {{.Lang.Name}} player and editor would. Find:

1. Mistranslations: meaning lost or changed.
2. Grammar: agreement, case, articles, plural forms
   ({{join .Lang.Plurals ", "}}){{if .Lang.Genders}}, the genders of nouns
   ({{join .Lang.Genders ", "}}) and the forms that agree with them{{end}}.
   `{who}` holds a nominative noun phrase with its article; it must read
   as the subject.
3. Glossary terms not used, or used inconsistently.
4. Style: too literal, too modern, too long for its `# max`, or out of
   tone with the rest.
5. Placeholders (`{n}`, `{who}`, ...) lost, added or misplaced.

## Style
{{if .Style}}
{{.Style}}
{{end}}
## Glossary

| English | {{.Lang.Name}} | Notes |
|---|---|---|
{{- range .Terms}}
| {{.En}} | {{.Forms}} | {{.Note}} |
{{- end}}

## Entries
{{range .Entries}}
{{range .Notes}}# {{.}}
{{end}}{{if .Max}}# max: {{.Max}}
{{end}}"{{.Key}}":
  en: {{.Source}}
  {{$.Lang.Code}}: {{.Current}}
{{end}}
## Your answer

Answer with one MAML object holding only the entries you would change,
`"key": value` with the dotted key quoted and the corrected
{{.Lang.Name}} value, each preceded by a `#` comment saying in English what was wrong. Answer `{}` if nothing
needs changing. No other text, no code fence. The object can be merged
as is with `go run ./cmd/i18n merge -lang {{.Lang.Code}}`.
