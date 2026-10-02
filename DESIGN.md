---
name: Gitgram
description: GitLab cards as classic Telegram Bot API HTML; one fixed title saying who did what in which repo on which ref, italic detail, folded proof, stage lines that carry the result, a rare tagline.
typography:
  headline:
    fontFamily: "inherit (Telegram client text face; plain line, handle in <b>)"
    fontWeight: 400
  handle:
    fontFamily: "inherit (Telegram client text face; <b> in the headline, <i> elsewhere)"
    fontWeight: 700
  small:
    fontFamily: "inherit (Telegram client text face, via <i>)"
    fontWeight: 400
  stage:
    fontFamily: "inherit (Telegram client text face; stage name in <b>, the rest plain)"
    fontWeight: 700
  fold-label:
    fontFamily: "inherit (Telegram client text face, via <b> on a quote's first line)"
    fontWeight: 700
  chip:
    fontFamily: "inherit (Telegram client monospace, via <code>)"
    fontWeight: 400
  block:
    fontFamily: "inherit (Telegram client monospace, via <pre><code class=\"language-log\">)"
    fontWeight: 400
  button:
    fontFamily: "inherit (Telegram client inline keyboard face)"
    fontWeight: 400
components:
  headline:
    typography: "{typography.headline}"
  detail-line:
    typography: "{typography.small}"
  stage-line:
    typography: "{typography.stage}"
  fold:
    typography: "{typography.fold-label}"
  commit-row:
    typography: "{typography.small}"
  ref-chip:
    typography: "{typography.chip}"
  log-block:
    typography: "{typography.block}"
  stop-button:
    typography: "{typography.button}"
  stop-confirm-button:
    typography: "{typography.button}"
  retry-button:
    typography: "{typography.button}"
---

# Design System: Gitgram

## Overview

**Creative North Star: "The Fixed Title"**

Gitgram owns no canvas, no colour and no font. Its surface is a classic Telegram Bot API HTML message (`<b>`, `<i>`, `<s>`, `<code>`, `<pre><code class="language-log">`, `<a>`, `<blockquote>`, `<blockquote expandable>`) plus, on cards that carry a pipeline, one row of callback buttons, drawn by whichever client and theme the reader chose. The design system is a text grammar built in `internal/render/render.go` from three kinds of line: the **headline**, written only by `headline(b, lead, prep, project, ref)`; **small lines**, italic, carrying the detail; and **folds**, expandable quotes whose bold first line is the label. Italic stands in for small text and a blank line stands in for a rule.

Every card opens with one headline that reads `@someone did this in|to repo (ref)`: the actor's handle in bold, what they did, a preposition, the linked project, and the ref in parentheses. The headline is the card's title and never changes once posted: a pipeline is `@emi ran pipeline #84 in demo (main)` from queued to failed; an MR is always its opening, `@ada opened MR !42 in demo (src → tgt)`, and merging or closing it adds small lines, never a new title. State lives below the title: in small fact lines, in the stage lines, in the buttons. Rendering is pure and deterministic, so identical state yields identical bytes and edits are hash-skipped.

The cards are text only. No emoji appears anywhere in rendered output: no status lamps, no button icons, no fact-line markers. No SHA appears anywhere either: a commit is its title, linked to the commit.

**Key Characteristics:**
- One fixed headline per card: bold unlinked handle, verb, preposition, linked project, ref in parentheses.
- Italic is the small-text voice: titles, fact lines, people, meta, diff stats, artifacts, the tagline.
- Pipeline state lives in the stage lines (an emoji per state plus words and a finish time); there is no status line.
- Only a failed stage gets a log: a bare `language-log` block right under it.
- Commits are linked titles, never SHAs; push commit rows are `title · @author` under the italic diff stats.
- Buttons are plain words: `Stop pipeline`, then `Yes, stop it` / `Keep running`; `Retry`.
- Roughly one card in ten closes with a line of house humour.

## Typography

**Display Font:** none owned (Telegram client text face)
**Body Font:** Telegram client text face, whatever the reader's platform and theme supply
**Label/Mono Font:** Telegram client monospace, via `<code>` and `<pre>`

**Character:** Three voices inside one client face: plain for the headline and the stage lines, italic for the quiet small print around them, bold only on the headline handle, stage names and fold labels. Monospace marks refs, job names, diff positions and logs.

### Hierarchy
- **Headline** (plain, bold handle): `lead + " " + prep + " " + linked project + " (" + ref + ")"`. Push: `@ada pushed to demo (feat/x)`, `@ada force-pushed to demo (feat/x)`, `@ada created a branch in demo (feat/x)`, `@ada deleted a branch in demo (feat/x)`. Tag: `@ada pushed a tag to demo (v13.5.0)`, `@ada deleted a tag in demo (v13.5.0)`. Pipeline: `@emi ran pipeline #84 in demo (main)`, `Pipeline #84 ran in demo (main)` with no triggerer. MR: `@ada opened MR !42 in demo (feat/x → develop)`, `opened draft MR` for drafts. Issue: `@grace opened issue #7 in demo`. Note: `@linus commented on !42 Fix the SSAID grant in demo`, the target dropped when the card replies to it; `edited a comment` on edits. Release (no actor in the payload): `Release 13.5.0 was published in demo (v13.5.0)`, `was updated`, `was deleted`. Deployment: `@ada deployed to production in demo (develop)`, the environment linked when it has a URL.
- **Small** (italic): one line per fact, written through `small()`, which flattens nested `<i>` so a handle never toggles italics off. The object's linked title (struck through on a closed MR, `(confidential)` on an issue), `Merged by @linus into develop`, `Closed by @ada`, the diff stats (`3 changed, 1 added • +42 −18`), `Pipeline #5981 failed · 2 manual`, `Conflicts with develop`, `2 unresolved threads` / `Discussions resolved`, `Approvals 0/1` while open, people (`Approvals 1/1 · Reviewers @linus (approved) · Labels client`), meta (`reopened by @ada`), `Force push: the branch history was rewritten`, deployment state (`Deploying...`, `Deployed`, `Failed`, `Waiting for approval`), `Deploy job`, `Assets: …`, `Artifacts: assemble 50.6 MB`, the tagline.
- **Stage** (mark, bold linked stage name, plain words, time): `✅ Build · 1:02`, `❌ Test: unit failed · 1:40`, `❌ Test: unit failed (stuck or timeout failure)`, `🏃 Deploy: running job staging...`, `✋ Deploy: waiting for deploy:prod`, `✋ Deploy: waiting...`, `➖ Cleanup: skipped`, `: canceled`, `: scheduled`, `⚠️ Lint: passed with warnings`. The stage name is sentence-cased and links to its running, else failed, else first job; job names are `<code>` chips; the time (`m:ss` or `h:mm:ss`, first start to last finish) appears only once every job in the stage is finished.
- **Fold label** (bold, first line inside the quote): `Description`, `1 comment`, `Message`, `Release notes`, `Jobs`. The push commits fold has no bold label; its first line is the italic diff stats. A note's body is an unlabelled expandable quote.
- **Chip** (monospace, `<code>`): branches and tags in the headline ref, job names, conflict targets, `path:line` diff positions.
- **Block** (monospace, `<pre><code class="language-log">`): the failed job's log tail, never inside a quote.

### Named Rules
**The Fixed Title Rule.** The headline is written once, by `headline()`, and never changes while the card is edited. State goes in the lines below it, never into the title.

**The Small-Voice Rule.** Italic means small text. Anything that is detail, meta or aside is a whole italic line, with the push fold's italic diff-stats row and the italic handles in fold rows as the only inner runs.

## Layout

A card is a vertical stack of self-contained lines joined by `\n`; `htmlfmt.Builder` treats each line as the unit of truncation, so every line carries balanced tags. The order is fixed: headline, small lines (title, outcome, diff stats, facts), folds, the stage block, the people and meta lines, then a blank line and the tagline when one is drawn. Slots a card has nothing for are omitted, never left blank.

Pipeline card: headline; the commit as a plain, non-expandable quote holding its linked title; a blank line; one stage line per stage in every state, the failed job's log right under its stage; `Downstream: agent #12 passed · …`; the verbose `Jobs` fold; a blank line; `Artifacts: …` in italics. Push card: headline; the force-push line when forced; the commits fold; then either the same stage block (after a blank line, with an italic artifacts line after another) when the push absorbs its pipeline, or one italic `Pipeline #n state` line when the pipeline has its own card. An absorbed pipeline repeats no commit; the fold above already lists it.

Density is phone-first. Commit rows fit 52 runes including the author, the pipeline commit quote 52, single commit lines 48; MR and issue titles clip at 72; comment excerpts at 120 runes and the first line; labels at 8 with `+N`. Commit folds end with a linked `+N more` to the compare view, or the branch history for a new branch.

Length is capped at Telegram's 4096 UTF-16 units less a 200-unit margin. `Builder.Truncate` shortens expandable quotes first, from the last one up, cutting at an inner line break when one falls in the second half of what fits and closing open tags; then it drops whole lines from the end and appends `… read more` linked to the object's URL.

### Named Rules
**The Blank-Line Rule.** The only separator is an empty line: before the stage block while a pipeline runs or when a push absorbs a finished pipeline, before the artifacts and update meta, and before a tagline. No rules, no dividers, no glyph rows.

**The Folds-First Rule.** When a card is too long, the expandable folds give way before any fact line does.

## Elevation & Depth

Flat. Depth is the client's own quote treatment: an expandable blockquote collapses the proof under its label, a plain blockquote holds the pipeline commit open, and a bare code block holds the failure log. There are no shadows, colours or surfaces the product controls.

### Named Rules
**The Failure Log Rule.** Only a failed stage gets a log: the hard-failed job's last lines, fetched once the job has failed, as a plain `language-log` block right under its stage. Running stages show no log and nothing polls a running job's trace. No `pre` sits inside a quote: Telegram Desktop pulls it out.

## Components

### Headline
Built only through `headline(b, lead, prep, project, ref)`. The lead is the bold handle from `o.who()` (`Someone` when the payload names nobody) and the verb phrase; `prep` is `in` or `to`; the project is its name linked to its page; the ref, when set, sits in parentheses. Branch refs are `<code>` chips linked to the branch tree (`branchRef`), the MR ref is two of them joined by ` → `, and tag refs (tag, release, tag pipeline) link to their tag page (`tagRef`); deployment refs are branch refs.

### Stage lines
The pipeline result. `stageMarks` in `emoji.go`, one emoji per stage state: ✅ passed, ⚠️ passed with warnings, ❌ failed, 🏃 running, ⏳ waiting, ✋ manual, 🕒 scheduled, ➖ skipped, 🚫 canceled. They are the only emoji on any card; titles, small lines and buttons stay emoji-free.

### Commit rows
A commit is `commitLine`: its title clipped to width and linked to the commit, nothing else. In the push fold each row is `title · @author` (the pusher's italic handle when the author name matches, else the author's italic name); the pipeline card quotes the commit title alone; release and deployment cards carry it as a small line.

### Folds
`<blockquote expandable>`, the label bold on the first line inside the quote, rows below. Comment rows are `@author: path:line linked excerpt`. Descriptions, tag messages, release notes and note bodies rewrite `@gitlab-user` into italic, unlinked handles.

### Handles
`@⁠username` with a word joiner (U+2060) after the `@` so Telegram never auto-links it, the display name when the payload has no username. Bold in the headline, italic everywhere else. Never a link.

### Buttons
- **Stop pipeline** while a pipeline is active; a press swaps it for **Yes, stop it** and **Keep running** in one row, the pending confirmation stored in `PipelineState.ConfirmStop` so later edits keep it.
- **Retry** once a pipeline failed, without confirmation.
- Drawn on pipeline cards and push cards absorbing a pipeline, only when the bot holds the write capability. Plain words, no icons, no URL buttons: links live inline on the anchor, ref, stage, commit or `+N more` they name.

### Tagline
One card in ten (`taglineOdds`) closes with a blank line and one italic line from `taglines.go`. Both the draw and the line come from an FNV hash of the card's seed (kind, project, object id), so a card keeps its tagline or its silence across edits. Notes, tag deletions and release deletions never draw one.

## Do's and Don'ts

### Do:
- **Do** open every card with one `headline(b, lead, prep, project, ref)` call: bold handle, what they did, `in` or `to`, linked project, ref in parentheses.
- **Do** keep the headline identical across every edit of a card; put outcomes (`Merged by …`, `Closed by …`, stage results) on the lines below.
- **Do** put every detail on its own italic line through `small()`.
- **Do** carry the pipeline result in the stage lines: the state's emoji, the state in words, the time once the stage finished.
- **Do** put a failed stage's log as a bare `language-log` block right under it.
- **Do** render commits as their linked title through `commitLine`, with `· @author` in the push fold.
- **Do** fold proof in `<blockquote expandable>` with the bold label as the first line inside the quote.
- **Do** join fragments and fold rows with ` · `; ` • ` only between file counts and line counts in the diff stats.
- **Do** build through `htmlfmt.Builder` and finish with `Truncate(limit, url)` so folds shorten before facts drop.

### Don't:
- **Don't** put an emoji anywhere: not on status, not on buttons, not on fact lines.
- **Don't** show a SHA anywhere; a commit is its linked title.
- **Don't** change a card's title after it is posted or add a status line above the stages.
- **Don't** show a log for a running or passed stage, or poll a running job's trace.
- **Don't** link a handle or drop the word joiner after its `@`.
- **Don't** put a log `pre` inside a quote.
- **Don't** add URL buttons; Stop (confirmed) while active and Retry once failed are the whole keyboard.
- **Don't** assume a colour, font or theme; everything must read in light and dark clients.
