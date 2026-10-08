---
name: translate
description: Translate Termablo or bring a translation up to date — prompt → translate → merge → check → review, with the shared glossary. Use for "translate the game into X", "/translate de", "update the translations", "add a language", or after English text in locales/en.maml changes.
---

Follow `docs/translating.md`: it is the runbook, with the file layout,
the entry format, the tool and the checks.

With a language code as the argument (`/translate ru`), bring that
language to zero missing and zero stale entries; with none, run
`go run ./cmd/i18n status` and do every language that is behind.

For each language:

1. `go run ./cmd/i18n prompt -lang <code> > <scratch>/prompt.md` and read
   it whole. It is the brief; translate as it says, as a native games
   localizer would, into `<scratch>/reply.maml` (split a big job into
   chunks, each a complete MAML object).
2. `go run ./cmd/i18n merge -lang <code> <reply>`; fix and re-merge what
   it rejects.
3. `go run ./cmd/i18n check -lang <code>`: 0 errors, 0 missing. Fix the
   warnings that are real; say why the others stay.
4. `go run ./cmd/i18n review -lang <code>`, review critically, merge
   the corrections.
5. `go test -count=1 -run 'LocalesValid|DrawEvery|Catalog|CodeKeys' .`

Languages are independent: translate several at once with one subagent
each, every one writing only `locales/<code>.maml` and its lock.

A term is decided in `locales/glossary.maml`, never in a single
translation: when a better word comes up, change the glossary and then
every line that uses the term. Do not edit `locales/en.maml` to suit a
translation; if the English is ambiguous, add a comment to it.
