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
- **Log tails**: one collapsible block with the last 10 lines of the failed job's log (open) or the running job's log (closed, refreshed every 15 s), syntax-coloured. Passed jobs leave no trace. Needs `gitlab.read_token`.
- **Artifacts**: every archive the jobs produced, linked with its size, once the pipeline ends.
- **Rich messages**: cards use Telegram's rich message format (Bot API 10.1+): a small lead line (`@ada ran a pipeline in agent`), a heading with the outcome and ref (`Pipeline #5981 failed on feat/x`), a three-column table of stages, jobs and clock times, one collapsible log (open on failure, closed while running), and small footer lines for the commit, the artifacts and the last update. Clients older than June 2026 cannot render them.
- **One button**: `Cancel`, shown only while a pipeline is active and gone with the edit that shows the outcome. Needs `gitlab.hooks_token`; anyone in the group may press. Everything else is a link in the text.
- **Merge request cards**: state, draft flag, assignees, reviewers, labels, approvals (N/M when the API allows it), unresolved thread count, head pipeline status, conflicts, and a merged/closed footer. Comments reply to the card; or fold them in with `collapse_notes` if your team comments like it's paid by the word.
- **Issue cards** with replying comments.
- **Push cards**: one small line (`↗︎ 7 commits pushed: feat/x • by @ada`), the commits in one quote, `N files changed · +A −D` folded over a `git diff --stat` block with per-file counts (via the API), and the pipeline for that push as it moves from queued to passed or failed, all edited in place. A newer push to the same branch gets its own card and marks the old one superseded. Branch created and deleted, and a force-push badge (detected via the API, because GitLab doesn't tell you).
- **Tags, releases, deployments** as plain messages with links.
- **Mentions**: `@gitlab-user` in a comment becomes a real Telegram mention through the `users` map.
- **Forum topics**: route pipelines, MRs and pushes to their own topics.
- **Reconciler**: cards that stopped receiving events get re-read from the GitLab API. GitLab does not retry failed webhook deliveries, ever, so somebody has to.
- **`sync-hooks`**: registers the webhook on every project in the group, or one group hook if you pay for Premium.
- Everything else is a tap away through the links in the card; there are no link buttons.

## Quick start

### 1. Telegram

1. `/newbot` at [@BotFather](https://t.me/BotFather), keep the token.
2. Add the bot to your group and make it an admin (reactions only reach admin bots, and topics need it too).
3. Get the chat id (`-100…`): forward a group message to [@getidsbot](https://t.me/getidsbot), or open the group in [web.telegram.org](https://web.telegram.org/a/) and read the URL fragment. Topic ids are the number after `_` in a topic's URL. The General topic needs no id.

### 2. GitLab

| Token | Scope | Used for | Required |
|---|---|---|---|
| `gitlab.read_token` | `read_api` | approvals, thread counts, force-push detection, log tails, reconciler | no, features degrade gracefully |
| `gitlab.hooks_token` | `api` | `gitgram sync-hooks` and the card operations (stop, retry, play, run) | only for sync-hooks and operations |
| `gitlab.webhook_secret` | any string | `X-Gitlab-Token` on each delivery | yes |

One token with `api` covers both. A group access token works too.

### 3. Run

Prebuilt multi-arch images (amd64, arm64) live at `ghcr.io/esauvisky/gitgram`. No clone needed:

```sh
mkdir gitgram && cd gitgram
curl -fsSLO https://raw.githubusercontent.com/esauvisky/gitgram/main/docker-compose.yml
curl -fsSL  https://raw.githubusercontent.com/esauvisky/gitgram/main/config.example.yaml -o config.yaml
# edit config.yaml: chat_id, group, threads, users
cat > .env <<EOF
GITGRAM_TELEGRAM_TOKEN=...
GITGRAM_TG_WEBHOOK_SECRET=$(openssl rand -hex 24)
GITGRAM_WEBHOOK_SECRET=$(openssl rand -hex 24)
GITGRAM_GITLAB_TOKEN=...
GITGRAM_GITLAB_HOOKS_TOKEN=...
EOF
docker compose up -d
docker compose exec gitgram /gitgram sync-hooks --config /config/config.yaml --dry-run
docker compose exec gitgram /gitgram sync-hooks --config /config/config.yaml
```

Put a TLS-terminating reverse proxy in front of `127.0.0.1:8080` and set `server.public_base_url` to whatever GitLab can reach. In `telegram.mode: polling` Telegram needs no public URL at all; GitLab still does.

Pin a version with `VERSION=0.1.0 docker compose up -d`. To build locally instead of pulling: clone the repo and `docker compose up -d --build`. Without Docker: `go run ./cmd/gitgram serve --config config.yaml --poll`, and set `storage.path` to somewhere writable.

## Configuration

`${VAR}` and `${VAR:-default}` are expanded before parsing. Unknown keys are rejected. Every validation error is reported in one go, not one per restart.

| Key | Default | Meaning |
|---|---|---|
| `telegram.token` | required | bot token |
| `telegram.chat_id` | required | target group (`-100…`), or several separated by commas (`-100…, -100…`); every card goes to each, edited in each. The first is the primary: forum topics (`threads`) apply there, the others get cards in General |
| `telegram.mode` | `webhook` | `webhook` or `polling` |
| `telegram.webhook_secret` | required in webhook mode | URL suffix and `secret_token` for Telegram updates |
| `telegram.threads` | | `<event class>: <topic id>`, plus `default` |
| `telegram.reactions` | `{stop: 👎, run: 🔥, play: 🫡}` | which reaction triggers which operation; must be Telegram reaction emoji |
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
| `defaults.pipelines.log_tail.lines` | `10` | log lines kept for each failed job; `0` disables tails |
| `defaults.pipelines.log_tail.live_lines` | `10` | log lines shown for each stage's running job |
| `defaults.pipelines.log_tail.interval` | `15s` | how often the running job's tail is refreshed (min `5s`); the loop ticks at the `defaults` value |
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

- Every pipeline, MR, issue and push owns exactly one Telegram message. Events update stored state; a single sender renders the latest state and edits the message. Identical renders are skipped by hash, so Telegram never sees "message is not modified" and you never see a pointless edit.
- Edits are debounced 700 ms: a burst of job events becomes one edit. First send and final state go out immediately.
- Telegram allows about 20 messages per minute per group, and edits count. The sender rate-limits (1/s, 20/min) and honours `retry_after`. A busy pipeline can delay other messages; it cannot drop them.
- Cards whose message was deleted by a human are marked and left alone. The bot does not resurrect things.
- State and the outbox live in SQLite; restarts resume pending edits, and the reconciler catches up on anything GitLab forgot to deliver.
- Retention: delivery keys 7 days, finished cards 30 days.

## Preview

`gitgram preview --config config.yaml` sends a mock card of every kind and scenario to the configured chat: pushes, branches, a pipeline going pending → running → failed with log tails and artifacts, a passing and a manual pipeline, a merge request opened → approved → merged with a reply, a draft closed, an issue, a tag, a release and a deployment. Cards go through the real engine and sender, so first sends, in-place edits, folding and reactions behave as in production. State lives in a temporary database, nothing touches GitLab, and Telegram is never polled, so it runs beside a live `serve`: `docker compose exec gitgram /gitgram preview --config /config/config.yaml`. Pick scenarios with `--scenario pipeline,mr` and pace them with `--delay 4s`. The same thing is one message away in the group: `/preview`, or `/preview pipeline mr`.

## Operations

`GET /healthz` pings SQLite. The container runs read-only as non-root, and `gitgram healthcheck` exists because distroless has no curl. On SIGTERM: stop accepting, drain 30 s, finish the in-flight Telegram call, close the database.

## Roadmap (v2)

- Per-user GitLab tokens linked from a DM, encrypted at rest, so operations run as the person pressing them instead of the bot's token.
- Approve and merge from the MR card.

## License

MIT.
