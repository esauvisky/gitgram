---
name: Gitgram
description: GitLab cards as Telegram rich messages in a five-line head: a small project line, one bold status line whose phrase names what matters, the object's title, a code chip for the ref, a small line for the person; then stages as a compact two-row-per-stage table with clock times on the right, one log fold, folded detail, and a plain two-wide keyboard.
typography:
  project:
    fontFamily: "inherit (Telegram client small text, via <footer>)"
    fontWeight: 400
  status:
    fontFamily: "inherit (Telegram client text face, via <p><b>)"
    fontWeight: 700
  title:
    fontFamily: "inherit (Telegram client text face, via <p>)"
    fontWeight: 400
  ref:
    fontFamily: "inherit (Telegram client monospace, via <code>)"
    fontWeight: 400
  person:
    fontFamily: "inherit (Telegram client small text, via <footer>)"
    fontWeight: 400
  stage:
    fontFamily: "inherit (Telegram client text face, via <td><b>)"
    fontWeight: 700
  clock:
    fontFamily: "inherit (Telegram client text face, via <td align=\"right\">)"
    fontWeight: 400
  body:
    fontFamily: "inherit (Telegram client text face, via <p> with <br/> lines)"
    fontWeight: 400
  live:
    fontFamily: "inherit (Telegram client text face, via <i>)"
    fontWeight: 400
  struck:
    fontFamily: "inherit (Telegram client text face, via <s>)"
    fontWeight: 400
  summary:
    fontFamily: "inherit (Telegram client text face, via <details><summary>)"
    fontWeight: 400
  log:
    fontFamily: "inherit (Telegram client monospace, via <pre><code class=\"language-log\">)"
    fontWeight: 400
  meta:
    fontFamily: "inherit (Telegram client small text, via closing <footer>)"
    fontWeight: 400
  button:
    fontFamily: "inherit (Telegram client inline keyboard face)"
    fontWeight: 700
---

# Design System: Gitgram

## Overview

**Creative North Star: "The Five-Line Head"**

Gitgram owns no canvas, no colour and no font. Its surface is a Telegram rich message (Bot API 10.1+, built block by block through `htmlfmt.Doc`) plus a normal inline keyboard, drawn by whichever client and theme the reader chose. The design system is a text grammar: which fact sits in which block, which block is small, which is bold, which is monospace, which folds, which is a button. The world is the user's own five mocks, transcribed in `.impeccable/surfaces/internal-render.md`, and the build follows them line for line.

Every pipeline, merge request and push card opens with the same head, read top to bottom in one glance on a phone: an `<h2>` project heading (`client / pokemod / agent`, the configured group stripped, segments joined by ` / `, the project name linked); a bold status line led by one lamp whose phrase names what matters (`❌ Pipeline #5981 · Failed in test`, `🟣 MR !42 · Merged`); the object's title as a plain paragraph; a `<code>` chip for the ref (`feat/ssaid-grant · MR !42`, `feat/ssaid-grant → develop`); and a small `<footer>` for the person (`Opened by Ada Lovelace`). The push card compresses that head into one line (`↗︎ 7 commits pushed: feat/ssaid-grant • by @ada`). Below the head the body is the object's evidence: a `<table compact>` of stages with a clock time right-aligned on each stage row and the jobs as plain linked text on the row beneath, one `<details>` log fold, an expandable quote for the first line of prose (description, commits), a `<details>` fold for the rest (`View changes and description`, `3 files changed · +42 −18`, `1 comment`), one paragraph of `<br/>`-separated facts on an MR, a pipeline line on a push. A closing `<footer>` holds the small print: who ran it, the commit as a 7-character code chip, the artifacts, `Updated HH:MM`. The keyboard is plain: two buttons per row, `↗︎` on every button that leaves Telegram, verbs bare (`Retry`, `Cancel`, `Play deploy:cdn`, `Run pipeline`).

The build refuses the category default: the emoji-stat rows and the sentence bot (`emi pushed 3 commits to develop in group/sub/project`), and it also refuses the previous world's devices: no `<h4>`, no `<mark>`, no `<ul>`, no in-text buttons, no bot reactions and no reaction controls. Status emoji appear only where the mocks draw them: on the status line, on stage rows, and on the fact lines that carry a state (`✅ Pipeline #5981 passed`, `🟣 Merged by Linus`, `💬 Discussions resolved`); a frozen push says `Superseded by a newer push` in plain text. Output is deterministic (same state, same bytes) so edits are hash-skipped. Note, issue, tag, release and deployment cards still speak classic Bot API HTML and are pending inheritors of this grammar.

**Key Characteristics:**
- Fixed five-line head on every rich card: project `<footer>`, bold status `<p>`, title `<p>`, `<code>` ref `<p>`, person `<footer>`; slots a card has nothing for are omitted, never left blank.
- One bold run per card, and it is the status line: lamp, space, `<b>Object anchor · Phrase</b>`; the phrase names the stage that failed or is running, the MR state, or what the push did, with the stage name in its own case and only the phrase's first letter capitalised (`sentence()`).
- One separator: ` · ` (space, middle dot, space) joins fragments everywhere (status line, ref line, footers, summaries, fact lines); ` / ` joins only the project path; ` → ` joins only source and target.
- Stages are a `<table compact>` with two rows per stage: `lamp <b>Stage</b>` with the clock (`m:ss`) right-aligned, then the jobs as plain text with linked names; table cells take inline text only.
- One log fold per pipeline card: `Error · last lines` open once failed, `Logs · current stage` closed while running; the log is `<pre><code class="language-log">`.
- Two folds: `<blockquote expandable>` for one line of prose (the MR description's first line, the commit list) and a closed `<details>` for the rest: `View changes and description` on an MR, `3 files changed · +42 −18` on a push, `1 comment`, `Jobs`.
- `<code>` carries refs, file names, diff positions and the 7-character commit SHA chip inside its link; `<i>` marks only the running job; `<s>` marks only what will not happen (skipped and canceled jobs, a closed MR's title, a deleted branch, a deleted file).
- Two `<footer>`s of small print: the project line at the top and the closing meta line (`who · commit <code>8f6ded0</code> · 📦 Artifacts: … · Updated HH:MM`), plus the person line under the ref chip.
- A normal inline keyboard, two buttons per row, `↗︎` (U+2197 U+FE0E, text presentation) on every URL button and never on a callback button; a card with nothing to do has no keyboard.
- Lamps are the only emoji: pipeline and job states (✅ ❌ ⚠️ 🔄 ⏳ ⏸️ ⏭️ 🚫 🕒), MR states (🟢 📝 🟣 🔴), push kinds (↗︎ 🌱 🗑️, ⚠️ force), and four markers (💬 threads, ⚔️ conflict, 📦 artifacts, 🏷️ tag ref) with the two relation arrows (↳ downstream, ↰ parent).

## Colors

The product owns no colour. Text, link, code, quote, table, details and footer tints belong to the Telegram client and its theme; both light and dark themes must read without any assumption about hue (the review rasters `mobile.png`, `mobile-light.png`, `desktop.png` are CSS approximations of both). The `language-log` hint on the log block lets clients colour the log on their own palette. The only chroma the renderer places is the lamp vocabulary in `emoji.go`, and every lamp is a signal beside a word.

### Lamp vocabulary (`emoji.go`, `statusEmoji`, `mrStateEmoji`)
- **✅ Passed**, **❌ Failed**, **⚠️ Passed with warnings** (a passed pipeline with allowed failures) / **Failed (allowed)** / **Force-pushed**, **🔄 Running** / **Canceling**, **⏳ Pending** (created, pending, preparing, waiting for resource or callback; `queued` on a pipeline line), **🚫 Canceled**, **⏸️ Waiting for manual**, **🕒 Scheduled**, **⏭️ Skipped**.
- **🟢 Open**, **📝 Draft** (open and draft), **🟣 Merged**, **🔴 Closed**. The same glyph leads the status line and the MR fact line (`🟣 Merged by Linus into <code>develop</code>`, `🔴 Closed by X`).
- **↗︎** push (`↗︎ 7 commits pushed`, `↗︎ Pushed` when the count is unknown; U+FE0E keeps it a text glyph, the same one that ends URL buttons), **🌱** branch created, **🗑️** the one-shot `Branch deleted` message, **⚠️** force push.
- **💬** `N unresolved threads` or `Discussions resolved`; **⚔️** merge conflict; **📦** artifacts; **🏷️** a tag ref on the ref chip; **↳** downstream pipelines; **↰** parent pipeline.

### Named Rules
**The Lamp Beside Its Word Rule.** A lamp never stands alone and never trails. On the status line it precedes the bold run; on a stage row it precedes the bold stage name; on a fact line it precedes the phrase it judges (`✅ Pipeline #5981 passed`, `💬 2 unresolved threads`, `⚔️ Conflicts with develop`). A reviewer's approval is a word, `Linus (approved)`, not a glyph.

**The Lamps Are The Only Emoji Rule.** Status lamps on status lines and stage rows, state lamps on fact lines, four markers and two arrows. Frozen-push lines (`Superseded by a newer push`, `Branch deleted`) are plain text. No decorative glyphs, no person or people icons, no glyphs on buttons, no reactions. `👤 👥 🏷 🔗 🔒` exist in `emoji.go` for the pending inheritors and appear on no rich card.

## Typography

**Display Font:** none owned; the Telegram client text face.
**Body Font:** none owned; the Telegram client text face.
**Label/Mono Font:** the Telegram client monospace via `<code>` (refs, SHA chips, files) and `<pre><code class="language-log">` (the log fold).

**Character:** Emphasis is structural. Small (`<footer>`) is context: the project at the top, the person under the ref, the meta at the bottom. Bold is the one status line and the stage names a scanner reads down. Plain is the title and every fact. Monospace is anything copy-pasteable into git. Italic is the one job that is moving. Strike-through is what will not happen.

### Hierarchy
- **Project** (small, `<footer>`): block one of every card, `Options.header`: the project path below `Options.Group` with directories plain and ` / ` between segments, the project name linked (`client / pokemod / <a>agent</a>`).
- **Status** (bold, `<p>lamp <b>…</b></p>`): the one bold run. Pipeline: `Pipeline <a>#n</a> · <phrase>` where the phrase is `Failed in <stage>`, `<stage> running`, `Passed`, `Passed with warnings`, `Waiting for manual: <stage>`, `Queued`, `Canceled`, `Failed`, `Skipped`, `Scheduled` (`pipelinePhrase` through `sentence()`: the stage keeps its own case, the first letter is capitalised, so `Failed in test` and `Deploy running`). MR: `MR <a>!n</a> · Open|Draft|Merged|Closed`. Push: `<N> commits pushed`, `Pushed`, `Force-pushed <N> commits`, `Branch created · <N> commits`, `Branch deleted`.
- **Title** (plain, `<p>`): the commit title on a pipeline (omitted when empty) and the MR title, clipped at 72 runes with an ellipsis (`maxTitleLen`); `<s>` when the MR is closed. A push has no title.
- **Ref** (monospace, `<p><code>`): pipeline `<code>ref</code> · MR <a>!n</a> · ↰ child of <a>#id</a>` (a tag ref prefixed `🏷️ `); MR `<code>source → target</code>` as one chip; push `<code>branch</code>`; a deleted branch `<s>branch</s>` with no code.
- **Person** (small, `<footer>`): `Opened by <who>` on an MR; the pusher's name alone on a push; a `tg://user?id=` mention when the GitLab username is mapped.
- **Stage** (bold, `<td>lamp <b>Stage</b></td>`): the stage name through `sentence()` with its aggregated lamp; never linked.
- **Clock** (`<td align="right">`): `htmlfmt.Clock`, `m:ss` or `h:mm:ss` (`1:02`, `0:30`); `—` for a stage that was wholly skipped or canceled; empty until something has finished.
- **Jobs row** (plain text, second `<td>` row): job names linked to their job page, ` · `-joined; failed jobs first with their reason in parentheses when it is not `script_failure`; `(allowed to fail)`, `(manual)` suffixes; a uniform stage is one word: `skipped`, `canceled`, `waiting`, `manual: deploy:cdn, deploy:prod`. Table cells take inline text only, so the jobs row is never a `<footer>` or a block.
- **Body** (plain, `<p>` with `<br/>` lines): the MR fact lines, the push pipeline line, the frozen line, the downstream line, the quiet summary, the paragraphs inside a fold.
- **Live** (italic, `<i>`): the running job on the jobs row. Nothing else is italic.
- **Struck** (`<s>`): skipped and canceled jobs on a mixed jobs row, a closed MR's title, a deleted branch, a deleted file in a diff list.
- **Summary** (`<details><summary>`): a label, counted when it can be: `Error · last lines`, `Logs · current stage`, `View changes and description`, `3 files changed · +42 −18`, `1 comment`, `Jobs`.
- **Log** (`<pre><code class="language-log">`, `htmlfmt.Pre(text, "log")`): the last lines of one job, only inside the log fold.
- **SHA chip** (`shaChip`): `<a href><code>8f6ded0</code></a>`, the first 7 characters, the code inside the link; unlinked `<code>` when the commit has no URL. Used in the commit list and the pipeline meta footer.
- **Meta** (small, closing `<footer>`): pipeline `<who> · commit <a><code>8f6ded0</code></a> · 📦 Artifacts: <a>job</a> 50.6 MB · Updated 04:25`; MR `<change> by <who> · Updated 04:43` (change only for edits no fact line conveys); push `Updated 04:42`. The one-shot branch-deleted message has none.
- **Button** (inline keyboard): one callback button only, `Cancel`, while a pipeline is active and `Options.Caps` allows. Links live in the text, never in buttons.

### Named Rules
**The One Bold Line Rule.** `<b>` wraps the status line's run (`Pipeline #5981 · Failed in test`) and the stage names in the table; the only other bold is a note author inside the comments fold (`<b>Linus</b>:`) and a stage header inside the verbose `Jobs` fold. The title, ref, footers, fact lines and buttons are never bold.

**The Phrase Names The Stage Rule.** The status phrase says where, not just what: `Failed in test`, `Deploy running`, `Waiting for manual: deploy`. Stage names keep the case GitLab gave them; `sentence()` capitalises only the first letter of the phrase, and the same function capitalises the stage name in the table so `Failed in test` points at the `Test` row.

**The Middle Dot Rule.** ` · ` is the only fragment separator: on the status line, the ref line, the footers, the fact lines, the summaries and the jobs row. The project path uses ` / ` and the branch movement ` → `; nothing uses ` • `, ` - `, `|` or `,` between fragments (commas belong inside a list of names: `manual: deploy:cdn, deploy:prod`, `Reviewers Linus, Tom`).

**The Chip Is One Code Run Rule.** The ref line's chip is a single `<code>` span: `feat/ssaid-grant`, or `feat/ssaid-grant → develop` with the arrow inside the code. Anchors beside it (`MR !42`, `child of #9`) are plain links outside the code. A commit is the opposite nesting: the `<code>` chip sits inside the `<a>`.

## Layout

The unit is the block (`htmlfmt.Doc`): `<footer>`, `<p>`, `<table>`, `<details>`, `<blockquote>`, joined by newlines; there is no wrapping control, so every line leads with the fragment that matters and trails with the disposable one. Every rich card is head, body, meta footer, keyboard.

**Head (all cards):** `<footer>project</footer>` → `<p>lamp <b>status</b></p>` → `<p>title</p>` (pipeline, MR) → `<p><code>ref</code>…</p>` → `<footer>person</footer>` (MR, push).

**Pipeline card** (`pipeline.go`):
1. Project footer.
2. Status: `❌ <b>Pipeline <a>#5981</a> · Failed in test</b>`.
3. Commit title (when known).
4. Ref line: `<code>feat/ssaid-grant</code> · MR <a>!42</a> · ↰ child of <a>#9</a>` (fragments present only when known).
5. `<table compact>`, two rows per stage in stage order: `lamp <b>Stage</b>` | clock, then jobs | empty. Quiet mode replaces it with one line: `4 jobs · 3 passed · 1 failed`. Verbose mode adds a closed `<details>Jobs</details>` fold with `<b>stage</b>` headers and `lamp <a>job</a> word · 1:02 · queued 0:03 · reason · retry N` rows.
6. `↳ Downstream: <a>infra #55</a> passed · <a>api #56</a> running (2 failed)` when children exist; quiet mode alone adds `⏸️ Waiting for manual: job, job` when blocked.
7. The one log fold: `<details open><summary>Error · last lines</summary>` with the first hard-failed job's tail once the pipeline is terminal; `<details><summary>Logs · current stage</summary>` with the running job's tail while it runs; none otherwise.
8. Closing footer: `<who> · commit <a><code>8f6ded0</code></a> · 📦 Artifacts: <a>assemble</a> 50.6 MB · Updated 04:25`.
9. Keyboard: `Cancel` alone while the pipeline is active and `Options.Caps` allows; nothing otherwise.

**Merge request card** (`mr.go`):
1. Project footer.
2. Status: `🟢 <b>MR <a>!42</a> · Open</b>`.
3. Title, `<s>` when closed.
4. `<code>feat/ssaid-grant → develop</code>`.
5. `<footer>Opened by <who></footer>`.
6. The description's first line as `<blockquote expandable>`, clipped at 200 runes, `@mentions` rewritten (config `mr.show_description`).
7. One `<p>` of `<br/>` lines, the lines the mock draws, in this order and only when true: `🟣 Merged by X into <code>tgt</code>` / `🔴 Closed by X`; `<lamp> Pipeline <a>#n</a> <word>` (`⏳ … queued` while pending, ` · N failed`, ` · N manual`); `⚔️ Conflicts with <code>tgt</code>`; `💬 N unresolved threads` or `💬 Discussions resolved` whenever threads are enriched, in every state; `Approvals n/m · names` while the MR is open.
8. `<details><summary>View changes and description</summary>`, closed: `<p>` full description (`<br/>` newlines, mentions rewritten); `<p>3 files changed · +42 −18<br/><code>file</code>…</p>` from `MRState.Diff` (the same `diffParts` as a push: rename `<code>old</code> → <code>new</code>`, `(new)`, deleted struck, `+N more`, `(partial)`); `<p>` of `Approvals n/m · names` (merged or closed only), `Reviewers Linus (approved), Tom`, `Assignees …`, `Labels a, b +N` (eight shown). Absent when every part is empty.
9. `<details><summary>N comments</summary><p><b>Author</b>: <code>path:line</code> <a>excerpt</a><br/>…</p></details>`, excerpt first line only, 120 runes.
10. Closing footer: `<change> by <who> · Updated 04:43`; the change fragment only for reopened, approved, draft/ready, title, description, labels, assignees, reviewers, threads, target, milestone, confidentiality, due date.
11. Keyboard: none; the anchor `!42` links the MR.

**Push card** (`push.go`, state in `cards/push.go`: one card per push, edited as diff stats and the head pipeline arrive, frozen when superseded). Below the heading and the title line everything is small print, and blocks stack with no gap.
1. Project heading.
2. `<p>↗︎ <b>7 commits pushed</b>: <a>feat/ssaid-grant</a> • <code>by @ada</code></p>` (branch linked to its tree page; the by-line a code chip, mention-linked when mapped). (branch linked to its tree, pusher as a mention or `@username`); variants `⚠️ <b>Force-pushed 7 commits</b>`, `🌱 <b>Branch created · 7 commits</b>`, `<b>Pushed</b>` when the count is unknown.
3. Commits as one `<blockquote expandable>`: `<a><code>8f6ded0</code></a> title - author` per commit, title clipped at 48 runes so the line holds on a phone, capped by `push.max_commits`, then `+5 more`.
4. An `<hr/>`, then a small `<footer>3 files changed · +42 −18</footer>` over a `<table compact>` with one row per file: the path as a `<code>` chip (renamed as `old → new`, `(new)`, deleted struck) and `+A −D` right-aligned; `… N more files` past the 30 kept; `No file changes` alone when the compare returned nothing.
5. An `<hr/>`, then the footer pipeline line: `⏳ Pipeline <a>#5986</a> queued` → `❌ Pipeline <a>#5986</a> failed`.
6. Frozen footer line: `Superseded by a newer push` or `Branch deleted`.
6a. Tagline footer: one of forty house lines picked from the push's date (`taglines.go`), the same all day, changing daily.
7. Keyboard: none of its own; an absorbed active pipeline contributes its `Cancel`.

**Branch deleted** (`BranchDeleted`, one-shot): project footer, `🗑️ <b>Branch deleted</b>`, `<s>branch</s>`, pusher footer; no meta footer, no keyboard.

**Truncation:** `htmlfmt.RichLimit` is 30000 characters; whole blocks are dropped from the end until the card fits, never the first block.

### Named Rules
**The Same Head Rule.** Project, status, title, ref, person, in that order, on every rich card; a slot the object lacks is skipped, and the body never begins before the person line.

**The Clock Column Rule.** The stage table's last cell is `align="right"` and is emitted even when empty, so the clock times form one column the eye runs down; a skipped or canceled stage shows `—` there rather than nothing. Cells hold inline text only.

**The Mock's Lines Stay Out Rule.** Outside the MR's fold only the lines the mock draws appear: how it ended, its pipeline, conflicts, discussions, and approvals while open. Reviewers, assignees, labels, the diff and the full description live inside `View changes and description`.

**The Frozen Card Rule.** A superseded push card says so in a plain line above the footer; nothing above the line is rewritten.

**The One Button Rule.** A card carries a button only while pressing it does something: `Cancel` on an active pipeline. Destinations are links in the text.

## Elevation & Depth

There are no shadows and no layering the product controls. Depth is three client-drawn devices. `<blockquote expandable>` folds a line of prose (the MR description's first line, the commit list) behind a tinted band the reader taps open. `<details>` folds anything with a label: the log fold, the MR's `View changes and description`, the push diff fold, the comments fold, the verbose `Jobs` fold. `<pre><code class="language-log">` sets the log apart in a monospace panel clients may colour. State never sits inside a fold: the status line, the stage table and the fact lines say what failed or runs before any fold is opened.

### Named Rules
**The One Fold Opens Rule.** Only the log fold ever opens by itself, and only on failure (`Error · last lines`); while running it is closed (`Logs · current stage`); everything else folds closed.

**The Two Folds Rule.** A line of prose folds into `<blockquote expandable>`; a labelled thing folds into `<details>` whose summary is the count when there is one (`3 files changed · +42 −18`, `1 comment`) or the name of what it holds (`View changes and description`). If a reader must expand to learn whether something failed, the card is wrong.

**The Log Is Evidence Rule.** A `<pre>` block is only ever a job log, only ever inside the one log fold, only for the first hard-failed job or the running job; passed jobs have no log. Lines are ANSI-stripped, section markers dropped, capped by `pipelines.log_tail` (`lines`, `live_lines`, default 10).

## Shapes

The renderer draws no corners, borders or panels. Every silhouette is the client's: the bubble, the quote bar, the details chevron, the code chip, the compact table's cell padding, the keyboard's pill buttons. The one shape decision in the build is the right-aligned clock column of the stage table.

## Components

### Project line
`Options.header`: `<footer>client / pokemod / <a>agent</a></footer>`. `Options.Group` is stripped from the path; each remaining `/` becomes ` / `; the project name is linked when its URL is known.

### Status line
`<p>lamp <b>…</b></p>`, the one bold line. Pipeline `Pipeline <a>#n</a> · <phrase>` with the phrase through `sentence()`; MR `MR <a>!n</a> · <State>`; push `<N> commits pushed` and its variants; branch deleted `Branch deleted`. The anchor is linked inside the bold.

### Title
`<p>` with the commit or MR title clipped at 72 runes (`clip`); `<s>` on a closed MR.

### Ref chip
`<p><code>…</code>…</p>`: the branch or tag (`🏷️ ` prefix on a tag), the MR movement as one code run, then ` · MR <a>!n</a>` and ` · ↰ child of <a>#id</a>` on a pipeline.

### Person line
`<footer>Opened by <who></footer>` on an MR; `<footer><who></footer>` on a push and on the branch-deleted message. A mapped user is a `tg://user?id=` link; unmapped is plain; nameless is `@username`; nothing at all is `someone`.

### Stage table
`htmlfmt.Table` → `<table compact>`, two rows per stage (`stageRows`): `<td>lamp <b>Stage</b></td><td align="right">clock</td>` then `<td>jobs</td><td align="right"></td>`. The lamp aggregates (running > queued > failed > canceled > manual > scheduled > skipped > warning > passed). The clock is `stageDuration` through `htmlfmt.Clock`: first start to last finish once every job is terminal, else the sum of finished durations, else empty; `—` for a wholly skipped or canceled stage. Jobs are plain inline text, linked names ` · `-joined, hard failures first with `(reason)` unless `script_failure`, the running job `<i>`, skipped and canceled `<s>`, `(allowed to fail)` and `(manual)` suffixes; uniform stages collapse to `skipped`, `canceled`, `waiting`, `manual: a, b`.

### Log fold
`htmlfmt.Details(summary, htmlfmt.Pre(lines, "log"), open)`: `Error · last lines` open for the first hard-failed job with a tail once terminal; `Logs · current stage` closed for the running job's tail; at most one per card; none in quiet mode.

### Expandable quote
`<blockquote expandable>`: the MR description's first line clipped at 200 runes (mentions rewritten), or the commit list of `<br/>`-joined rows (`<a><code>sha</code></a> title · author`, `+N more`).

### View changes and description
The MR's closed `<details>` (`mr.go`): `<p>` full description, `<p>` diff summary and file chips (`diffParagraph`), `<p>` of `Approvals` (merged or closed), `Reviewers` with `(approved)` after an approver, `Assignees`, `Labels`, each part present only when it has content.

### Counted fold
`<details><summary>label</summary><p>row<br/>row</p></details>`, closed: the push diff `3 files changed · +42 −18` with `<code>file</code>` rows (rename `old → new`, `(new)`, deleted struck, `+N more`, `(partial)`); `N comments` with `<b>Author</b>: <code>path:line</code> <a>excerpt</a>` rows; verbose `Jobs`.

### Fact lines
One `<p>` of `<br/>`-separated lines on an MR (merged/closed, pipeline, conflicts, discussions, approvals while open); a lone `<p>` pipeline line on a push (`pipelineLine`, shared); a lone plain `<p>` frozen line (`Superseded by a newer push`, `Branch deleted`); `↳ Downstream: …` on a pipeline.

### SHA chip
`shaChip(sha, url)`: `<a href="…"><code>8f6ded0</code></a>`, 7 characters, the code nested inside the link; bare `<code>` without a URL. In the commit list and the pipeline meta footer (`commit <a><code>8f6ded0</code></a>`).

### Meta footer
Closing `<footer>` of ` · ` fragments: pipeline `<who> · commit <sha chip> · 📦 Artifacts: <a>job</a> <size> · Updated HH:MM` (size via `htmlfmt.Size`: `512 B`, `1 KB`, `50.6 MB`); MR `<change> by <who> · Updated HH:MM`; push `Updated HH:MM`. `Updated` is the card's last event time in `Options.Location`, never the wall clock.

### Keyboard
`Message.Keyboard` holds at most one row with one callback button (`actions.Callback`): `Cancel` while a pipeline is active and `Options.Caps` allows. Editing a finished pipeline omits the markup, which removes the button.

## Do's and Don'ts

### Do:
- **Do** open every rich card with the `<footer>` project line (`client / pokemod / <a>agent</a>`, group stripped, ` / ` between segments) and then the bold status line led by its lamp.
- **Do** make the status phrase name the stage in the stage's own case, capitalising only the first letter through `sentence()`: `Failed in test`, `Deploy running`, `Waiting for manual: deploy`.
- **Do** keep the head order project, status, title, ref chip, person, and omit a slot the card has nothing for.
- **Do** join fragments with ` · ` everywhere, ` / ` in the project path, ` → ` between branches inside one `<code>` chip.
- **Do** render stages as `<table compact>` with two rows per stage, the clock (`htmlfmt.Clock`, `m:ss`) right-aligned on the stage row and the jobs as plain inline text with linked names on the row beneath; `—` for a skipped or canceled stage.
- **Do** list hard-failed jobs first with their reason unless it is `script_failure`, italicise the running job, strike skipped and canceled ones, and collapse a uniform stage to one word.
- **Do** carry exactly one log fold: `Error · last lines` open on failure, `Logs · current stage` closed while running, the log in `<pre><code class="language-log">`.
- **Do** quote only the description's first line (200 runes) and fold the full description, the diff and the people into a closed `View changes and description`.
- **Do** keep outside the MR fold only the mock's lines, in order: merged/closed by whom, pipeline, conflicts, `💬` discussions whenever enriched, approvals while open; use the shared `pipelineLine` for the pipeline on MR and push cards.
- **Do** write a commit SHA as a 7-character `<code>` chip inside its link (`shaChip`), in the commit list and the pipeline footer.
- **Do** strike what will not happen: skipped and canceled jobs, a closed MR's title, a deleted branch, a deleted file.
- **Do** close with a `<footer>` ending in `Updated HH:MM` (pipelines also `who · commit <sha chip> · 📦 Artifacts:`), and leave the one-shot branch-deleted message without one.
- **Do** draw the keyboard two wide, destination first, ` ↗︎` (U+2197 U+FE0E) on every URL button, bare verbs on callbacks, and drop `Create MR ↗︎` and `Run pipeline` from a superseded push.
- **Do** keep output deterministic: same state, same bytes; drop whole blocks from the end only past `htmlfmt.RichLimit`.

### Don't:
- **Don't** use `<h4>`, `<mark>`, `<ul>`, in-text `<tg-button>`s, bot reactions or reaction controls; the previous world is gone.
- **Don't** bold anything but the status run, the stage names, a note author in the comments fold and a stage header in the verbose fold.
- **Don't** italicise anything but the running job, or strike anything that may still happen.
- **Don't** put a lamp anywhere without the word it judges, trail a name with a glyph (`(approved)` is a word), add a glyph to a button, put a lamp on a frozen-push line, or use any emoji outside the lamp vocabulary (`👤 👥 🏷 🔗 🔒` are for the pending inheritors only).
- **Don't** write the person on the status line or the title; the person is a `<footer>` under the ref chip and the meta footer at the bottom.
- **Don't** put a block, a `<footer>` or a `<p>` inside a table cell; cells take inline text only.
- **Don't** link stage names, put a job's log anywhere but the one log fold, show a log for a passed job, or open a fold other than the failure log.
- **Don't** put reviewers, assignees, labels, the diff or the full description outside the MR's `View changes and description` fold.
- **Don't** use ` • `, ` - `, `|` or `,` as a fragment separator, put the ` → ` outside the `<code>` chip, or use the emoji-presentation `↗️` where the text glyph `↗︎` belongs.
- **Don't** hide state inside a quote or a details block; the status line and the table must say what failed before anything is expanded.
- **Don't** draw a callback button without `Options.Caps`, more than two buttons on a row, more than three log buttons, or a log button for the job whose log is already on the card.
- **Don't** write a prose sentence (`emi pushed 3 commits to develop in …`) or an emoji-stat row on a rich card.
