---
version: 1
slug: "internal-render"
primary_target: "internal/render"
related_targets: ["internal/render/pipeline.go","internal/render/mr.go","internal/render/push.go","internal/render/issue.go","internal/render/note.go","internal/render/release.go","internal/render/deployment.go"]
---

# Surface: Telegram cards (all kinds)

Scope: every Telegram message `internal/render` produces: pipeline, merge request, push, branch deleted, tag, issue, note, release, deployment. Visitor mode: Operate.

Audience and job: a developer glancing at the team group on phone or desktop, reading state, branch, project and person, then following a link into GitLab. Content is the real GitLab facts in card state; nothing is invented.

Constraints: classic Bot API HTML only (b, i, u, s, spoiler, a, tg://user, code, pre with language, blockquote, blockquote expandable), 4096 characters, literal newlines, both client themes, three verbosity levels, deterministic output for hash-skipped edits. No rich message API: no headings, footer, table, details, hr. One callback button, Cancel, while a pipeline is active.

Memorable moment: the lead line says who did what where; the bold line under it says what state it is in.

## Direction contract
THESIS: One fixed title per card in the form `@someone did this in repo (branch)`, then only the facts that change underneath it. It refuses status lines, emoji outside the stage marks, SHAs and live logs.
OWN-WORLD: classic Bot API HTML; the title has a bold handle (word joiner after @, never a link), a linked project and a linked code chip for the ref in parentheses, and is never edited after posting; italic is the small text; expandable quotes are the folds (bold first-line label, or the italic diff stats on push commits); stage lines use one emoji per state with the bold linked stage name, its state in words and its time once finished; only a failed stage gets a `language-log` block under it; commits appear as linked titles, never SHAs; no emoji anywhere; buttons are plain words (Stop pipeline with a Yes, stop it / Keep running confirmation, Retry on failure).
STORY: who did what where, in one line that never changes; the commits; where the pipeline is or why it failed; a link for everything else.
FIRST VIEWPORT: `@ada pushed to demo (feat/x)`; the commits quote headed by `3 changed • +42 −18`; a blank line; `✅ Build · 1:02`, `❌ Test: unit failed · 1:40` with its log, `➖ Deploy: skipped`; the Retry button. Pipeline card: `@emi ran pipeline #84 in demo (main)`, the commit quoted, the stage lines, then `Artifacts: …`.
FORM: user-pinned through iterative direction in the live test group (classic HTML, fixed titles, no emoji, no SHAs, no live logs); no seed roll, pinned brief beats the roll.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.
