<p align="center">
  <img src="assets/logo.png" width="160" alt="Gitgram logo: a git graph merging into a paper plane">
</p>

<h1 align="center">Gitgram</h1>

<p align="center">GitLab webhooks → one Telegram group. Cards that edit themselves instead of forty messages nobody reads.</p>

---

Gitgram is a self-hosted bot that takes every project under one gitlab.com group and turns its webhooks into Telegram messages that stay useful. A pipeline is **one message** that updates as jobs run. A merge request is **one message** that tracks approvals, threads, conflicts and its pipeline. Comments reply to the card they belong to, so the group chat reads like a timeline instead of a log file.

It exists because [Integram](https://github.com/requilence/integram) was archived, GitLab's built-in Telegram integration sends one flat line per event and calls it a day, and every other bot on GitHub either stopped in 2022 or only does DMs.

Single static Go binary. SQLite. No cgo. Runs happily in a 20 MB distroless container.

## What you get

- **Pipeline cards**: pending → running → done, edited in place. Stage rows, job names, failed jobs with log links, durations, "waiting for manual job" states, child pipelines folded into the parent. Retries don't spawn a new card, they update the old one.
- **Merge request cards**: state, draft flag, assignees, reviewers, labels, approvals (N/M when the API allows it), unresolved thread count, head pipeline status, conflicts, and a merged/closed footer. Comments reply to the card; or fold them in with `collapse_notes` if your team comments like it's paid by the word.
- **Issue cards** with replying comments.
- **Push summaries** with commit list, branch created/deleted, and a force-push badge (detected via the API, because GitLab doesn't tell you).
- **Tags, releases, deployments** as plain messages with links.
- **Mentions**: `@gitlab-user` in a comment becomes a real Telegram mention through the `users` map.
- **Forum topics**: route pipelines, MRs and pushes to their own topics.
- **Reconciler**: cards that stopped receiving events get re-read from the GitLab API. GitLab does not retry failed webhook deliveries, ever, so somebody has to.
- **`sync-hooks`**: registers the webhook on every project in the group, or one group hook if you pay for Premium.
- Read-only toward GitLab in v1. Buttons open GitLab; the seam for "approve from Telegram" is built, just not armed.

## Quick start

### 1. Telegram

1. `/newbot` at [@BotFather](https://t.me/BotFather), keep the token.
2. Add the bot to your group. If the group has topics, make it an admin.
3. Get the chat id (`-100…`): forward a group message to [@getidsbot](https://t.me/getidsbot), or open the group in [web.telegram.org](https://web.telegram.org/a/) and read the URL fragment. Topic ids are the number after `_` in a topic's URL. The General topic needs no id.

### 2. GitLab

| Token | Scope | Used for | Required |
|---|---|---|---|
| `gitlab.read_token` | `read_api` | approvals, thread counts, force-push detection, reconciler | no, features degrade gracefully |
| `gitlab.hooks_token` | `api` | `gitgram sync-hooks` only | only for sync-hooks |
| `gitlab.webhook_secret` | any string | `X-Gitlab-Token` on each delivery | yes |

One token with `api` covers both. A group access token works too.

### 3. Run

```sh
cp config.example.yaml config.yaml          # edit chat_id, group, threads, users
cat > .env <<EOF
GITGRAM_TELEGRAM_TOKEN=...
GITGRAM_TG_WEBHOOK_SECRET=$(openssl rand -hex 24)
GITGRAM_WEBHOOK_SECRET=$(openssl rand -hex 24)
GITGRAM_GITLAB_TOKEN=...
GITGRAM_GITLAB_HOOKS_TOKEN=...
EOF
docker compose up -d --build
docker compose exec gitgram /gitgram sync-hooks --config /config/config.yaml --dry-run
docker compose exec gitgram /gitgram sync-hooks --config /config/config.yaml
```

Put a TLS-terminating reverse proxy in front of `127.0.0.1:8080` and set `server.public_base_url` to whatever GitLab can reach. In `telegram.mode: polling` Telegram needs no public URL at all; GitLab still does.

Without Docker: `go run ./cmd/gitgram serve --config config.yaml --poll`, and set `storage.path` to somewhere writable.

## Configuration

`${VAR}` and `${VAR:-default}` are expanded before parsing. Unknown keys are rejected. Every validation error is reported in one go, not one per restart.

| Key | Default | Meaning |
|---|---|---|
| `telegram.token` | required | bot token |
| `telegram.chat_id` | required | target group (`-100…`) |
| `telegram.mode` | `webhook` | `webhook` or `polling` |
| `telegram.webhook_secret` | required in webhook mode | URL suffix and `secret_token` for Telegram updates |
| `telegram.threads` | | `<event class>: <topic id>`, plus `default` |
| `server.listen` | `:8080` | listen address |
| `server.public_base_url` | required for webhook mode and sync-hooks | external base URL |
| `server.gitlab_webhook_path` | `/webhook/gitlab` | GitLab delivery path |
| `server.telegram_webhook_path` | `/webhook/telegram` | Telegram update path (secret appended) |
| `gitlab.base_url` | `https://gitlab.com` | instance URL |
| `gitlab.group` | required | top-level group; subgroups included |
| `gitlab.read_token` | | `read_api` token |
| `gitlab.hooks_token` | | `api` token for sync-hooks |
| `gitlab.webhook_secret` | required | `X-Gitlab-Token` value |
| `storage.path` | `/data/gitgram.db` | SQLite file |
| `logging.level` / `logging.format` | `info` / `text` | slog level; `text` or `json` |
| `defaults.events` | all | subset of `push tag pipeline mr mr_note issue issue_note release deployment` |
| `defaults.verbosity` | `normal` | `quiet` (final state only), `normal`, `verbose` (per-job lines, queued time) |
| `defaults.branches.allow` / `.deny` | `["*"]` / `[]` | globs (`release/*`) or `re:` regexps; deny wins |
| `defaults.pipelines.child_cards` | `inline` | `inline` (in parent card), `own`, `both` |
| `defaults.pipelines.quiet_success` | `true` | with `quiet`: stay silent on success |
| `defaults.mr.collapse_notes` | `false` | fold comments into the MR card instead of replying |
| `defaults.mr.show_description` | `true` | description as an expandable quote |
| `defaults.mr.show_system_notes` | `false` | relay GitLab system notes |
| `defaults.push.max_commits` | `10` | commits listed per push |
| `projects[]` | | `path: group/project` plus any `defaults` key and `threads` |
| `users` | | `gitlab_username: telegram_user_id` for mentions |

Events from projects outside `gitlab.group` are ignored unless listed in `projects[]`.

## Webhooks

- **Free tier**: `gitgram sync-hooks` walks every non-archived project under the group and creates or updates a project hook. Re-run it when projects are added. `--dry-run` shows the plan.
- **Premium/Ultimate**: `gitgram sync-hooks --group-hook` registers a single group webhook. Don't do both; every event arrives twice and you get to pay for deduplicating it.

Endpoint: `POST <public_base_url>/webhook/gitlab`. Events to enable: Push, Tag push, Issues, Confidential issues, Comments, Confidential comments, Merge request, Job, Pipeline, Deployment, Release.

## How cards behave

- Every pipeline, MR and issue owns exactly one Telegram message. Events update stored state; a single sender renders the latest state and edits the message. Identical renders are skipped by hash, so Telegram never sees "message is not modified" and you never see a pointless edit.
- Edits are debounced 700 ms: a burst of job events becomes one edit. First send and final state go out immediately.
- Telegram allows about 20 messages per minute per group, and edits count. The sender rate-limits (1/s, 20/min) and honours `retry_after`. A busy pipeline can delay other messages; it cannot drop them.
- Cards whose message was deleted by a human are marked and left alone. The bot does not resurrect things.
- State and the outbox live in SQLite; restarts resume pending edits, and the reconciler catches up on anything GitLab forgot to deliver.
- Retention: delivery keys 7 days, finished cards 30 days.

## Operations

`GET /healthz` pings SQLite. The container runs read-only as non-root, and `gitgram healthcheck` exists because distroless has no curl. On SIGTERM: stop accepting, drain 30 s, finish the in-flight Telegram call, close the database.

## Roadmap (v2)

- Per-user GitLab tokens linked from a DM, encrypted at rest.
- Action buttons: retry/cancel/play jobs, approve/merge MRs, executed as the person pressing them. The callback codec, capability gate and dispatcher already exist. Today every button politely says "Actions aren't enabled on this bot yet."

## License

MIT.
