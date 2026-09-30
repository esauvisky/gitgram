---
version: 1
slug: "internal-render"
primary_target: "internal/render"
related_targets: ["internal/render/pipeline.go","internal/render/mr.go","internal/render/push.go"]
---

# Surface: Telegram cards (pipeline, MR, push)

Scope: the live-edited Telegram cards rendered by `internal/render` (pipeline, merge request, push) and their inline keyboards. Visitor mode: Operate.

Audience and job: a developer on a phone or desktop glancing at the team's group chat, scanning for state, then tapping once into GitLab. Proof and content are the real GitLab facts already in card state; nothing is invented.

Constraints: Telegram rich message HTML (Bot API 10.1+: h1-h6, p, table, details/summary, footer, mark, pre with language, blockquote expandable, tg-button with danger/success/primary/link styles) for pipeline, MR and push; classic Bot API HTML for the pending inheritors; 32k characters, both client themes, three verbosity levels, deterministic output for hash-based edit skipping. Engine, store, outbox, reconciler untouched. Note, issue, tag, release and deployment cards inherit the grammar later.

Memorable moment: the first line is a bulletin, not a sentence: one bold status word, one anchor, one number, and nothing else.

Unresolved: short-name collisions between subgroups fall back to `parent/name`; no Threads button because GitLab has no stable discussions anchor.

## Direction contract

THESIS: The user's five mocks are the brief: a small project line, a bold status line whose phrase names what matters (`Pipeline #5981 · Failed in test`, `MR !42 · Merged`, `7 commits pushed`), the object's title, a code chip for the ref, dim footer lines for people and meta, a table of stages with clock times, one collapsible log, folded detail (commits, diff stats, description), and two plain buttons. It refuses the emoji-stat rows and the sentence bots.

OWN-WORLD: Telegram rich messages; status emoji only where the mocks draw them (title line and stage rows); `<footer>` is the small dim text; `<table compact>` aligns clock times right (its cells take inline text only, so the jobs row cannot be a small footer and is plain text with the job names linked); `<details>` folds logs, diff stats and comments; `<code>` chips carry refs, SHAs and file names; buttons are a normal keyboard, two per row, `↗` on links. No bot reactions, no reaction controls.

STORY: the reader sees which project, what happened and where it failed or is running in three lines, then the stages, then the reason, then acts with one of two buttons.

FIRST VIEWPORT: `client / pokemod / agent` small; `❌ Pipeline #5981 · Failed in test` bold; `Fix the SSAID grant`; `feat/ssaid-grant · MR !42`; table `✅ Build 1:02 / assemble`, `❌ Test 1:40 / unit`, `⏭ Deploy — / skipped`; `▾ Error · last lines` open with the log; footer `Ada Lovelace · commit 8f6ded0 · Updated 04:43`; buttons `Open pipeline ↗` `Retry`. Push: `↗ 7 commits pushed`, `feat/ssaid-grant`, `Ada Lovelace`, commits quote, `▸ 3 files changed · +42 −18`, `⏳ Pipeline #5986 queued`, `Compare ↗` `Create MR ↗`. MR: `🟣 MR !42 · Merged`, title, `feat/ssaid-grant → develop`, `Opened by Ada Lovelace`, description quote, `🟣 Merged by Linus into develop`, `✅ Pipeline #5981 passed`, `💬 Discussions resolved`, `Updated 04:43`, `Open MR ↗` `Discussion ↗`.

FORM: user-pinned mocks; the direction roll c1580cc3 was superseded by the pinned brief; code-led.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.
