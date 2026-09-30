# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

The design surface is not a browser page. Gitgram has no web UI. Its only user-facing surface is Telegram messages rendered from Bot API HTML (bold, italic, code, links, blockquotes, expandable quotes) plus inline keyboards, shown by Telegram clients on desktop and mobile in light and dark themes. `web` is the closest schema value; treat it as "text rendered by a client the product does not control".

## Users

Small development teams on gitlab.com Free tier who self-host the bot and live in one Telegram group (optionally with forum topics). They want CI, merge request, push and issue signal from every project under one GitLab group without the group chat turning into a log file. The operator who deploys and configures it is usually one of the same developers.

## Product Purpose

Gitgram turns GitLab webhooks into Telegram cards that edit themselves: one message per pipeline, one per merge request, one per issue, comments replying to the card they belong to, pushes as a single summary. It exists because Integram was archived, GitLab's built-in Telegram integration emits one flat line per event, and the remaining bots are abandoned or DM-only. Success is a group chat that reads like a timeline of what is happening across the team's projects, at a glance, with fewer messages rather than more.

## Positioning

Live-edited cards instead of an event stream. Every pipeline, MR and issue owns exactly one Telegram message; events update stored state and a single sender re-renders and edits it, deduplicated by render hash, debounced 700 ms, rate-limited to Telegram's per-group budget. A reconciler re-reads stale cards from the GitLab API because GitLab never retries failed webhook deliveries. Single static Go binary, SQLite, no cgo, runs in a 20 MB distroless container.

## Operating Context

- One Telegram group per deployment, `-100…` chat id, optional forum topics routed per event class (`pipeline`, `mr`, `default`, per-project overrides).
- Cards are read in passing: on a phone during the day, on desktop while waiting for CI. Readers scan for status, branch, project and who, then tap a button to open GitLab.
- Telegram is the renderer. Layout tools are limited to line breaks, bold/italic/code/links, blockquotes (including expandable ones), emoji, and inline keyboard buttons. No color control, no alignment, no images per card, message length capped (renderer truncates and links to the source URL). Client theme is the user's choice; dark and light must both read.
- Telegram allows roughly 20 messages per minute per group and edits count; busy pipelines can delay other cards but never drop them.
- Operator surface: `config.yaml` with `${VAR}` expansion, `gitgram serve`, `gitgram sync-hooks --dry-run`, `GET /healthz`, Docker Compose, GHCR multi-arch images.
- Verbosity per project: `quiet` (final state only), `normal` (stage rows), `verbose` (per-job lines, queued time).

## Capabilities and Constraints

- Event classes: push, tag, pipeline, mr, mr_note, issue, issue_note, release, deployment. Branch allow/deny globs and regexps.
- Pipeline cards: pending → running → done edited in place, stage rows, failed jobs with up to three log buttons, durations, manual-job waiting state, child pipelines inline, own or both. Retries update the existing card.
- MR cards: state, draft flag, assignees, reviewers, labels, approvals (N/M when the API allows), unresolved thread count, head pipeline status, conflicts, merged/closed footer, description as expandable quote, optional collapsed notes and system notes.
- Issue cards with replying comments. Push summaries with commit list (max configurable), branch created/deleted, force-push badge via API.
- Mentions: `@gitlab-user` becomes a real Telegram mention via the `users` map.
- Read-only toward GitLab in v1. Every button opens GitLab. The action seam (callback codec, capability gate, dispatcher) exists; every action button currently replies "Actions aren't enabled on this bot yet."
- Emoji vocabulary is centralized in `internal/render/emoji.go` and shared across all cards; changing status semantics means changing it there.
- Cards deleted by a human are marked and never resurrected. Retention: delivery keys 7 days, finished cards 30 days.
- Terminology: card (one owned message), outbox (pending sends/edits), reconciler, janitor, stage row, child/downstream pipeline, sync-hooks.
- Roadmap (v2, not built): per-user GitLab tokens linked from a DM, action buttons (retry/cancel/play jobs, approve/merge) executed as the pressing user.
- Undecided: no website or docs site is planned. Surface for this design context is the Telegram cards only.

## Brand Commitments

Name: Gitgram. The current logo (git graph merging into a paper plane, GitLab orange-red into Telegram blue on dark navy, `assets/logo.svg`, `assets/logo.png`, `assets/logo-128.png`, `assets/social-preview.png`) is an explicit placeholder and may be replaced. README voice is dry and direct with occasional deadpan humor ("comments like it's paid by the word", "somebody has to"); nothing about voice has been declared binding.

## Evidence on Hand

- Live screenshot from the user's own group (bot shown as `tg-git-notificator`, dark Telegram theme, 2026-09-30): push cards with author, commit count, branch and project, commit lines quoted in a blockquote with expand chevron, a `Compare` button; pipeline cards with status emoji, number, branch, short SHA, commit title, author, duration, stage rows, a `Pipeline` button; MR cards with state emoji, `!id` title, `source → target`, author, a `Merged by` footer, `MR` and `Changes` buttons; a comment card using Telegram's Reply affordance. Observed in that capture: two consecutive push cards for the same commits, an MR attributed to "by someone", and a `~` suffix on the author name.
- Renderer source: `internal/render/*.go`, `internal/render/htmlfmt/`, keyboards in `internal/actions/`.
- No testimonials, adoption numbers, customer names or benchmarks exist. Do not invent any.

## Product Principles

- One message per thing. A card is the current truth of its object; history lives in GitLab, not in the chat.
- Scan first, tap second. Status, project, branch and person must be readable in the first line on a phone; everything else is detail below.
- Fewer edits, never fewer facts. Debounce and hash-skip are allowed to delay, never to drop state the reader needs.
- Work inside Telegram's grammar. The renderer owns text, emoji, quotes and buttons; it does not fight the client's theme, wrapping or fonts.
- Degrade gracefully. Missing tokens or API scope remove details, never break a card.
