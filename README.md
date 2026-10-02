<p align="center">
  <img src="assets/logo.png" width="160" alt="Gitgram logo: a git graph merging into a paper plane">
</p>

<h1 align="center">Gitgram</h1>

<p align="center">GitLab webhooks → one Telegram group. Cards that edit themselves instead of forty messages nobody reads.</p>

---

Gitgram is a self-hosted bot that takes every project under one gitlab.com group and turns its webhooks into Telegram messages that stay useful. A pipeline is **one message** that updates as jobs run, and a push is **one message** that carries its commits and the pipeline it triggered, so the group chat reads like a timeline instead of a log file. Today it covers pushes and pipelines; the other GitLab events are listed under [Not supported yet](#not-supported-yet).

It exists because [Integram](https://github.com/requilence/integram) was archived, GitLab's built-in Telegram integration sends one flat line per event and calls it a day, and every other bot on GitHub either stopped in 2022 or only does DMs.

Single static Go binary. SQLite. No cgo. Runs happily in a 20 MB distroless container.

## What you get

- **Pipeline cards**: pending → running → done, edited in place. Stage rows, job names, failed jobs with log links, durations, "waiting for manual job" states, child pipelines folded into the parent. Retries don't spawn a new card, they update the old one.
- **Failure logs**: when a job fails, the last 10 lines of its log appear under its stage, syntax-coloured. Running and passed jobs show no log, just the stage line and, once finished, how long it took. Needs `gitlab.read_token`.
- **Artifacts**: every archive a stage's jobs produced, linked with its size at the end of the stage's line, once the pipeline ends.
- **One layout for every card**, in plain Telegram HTML that any client renders: a title that never changes once posted, `@someone did this in repo (branch)` (`@ada pushed to agent (feat/x)`, `@emi ran pipeline #84 in agent (main)`), italic detail lines, one line per pipeline stage (`✅ Build · 1:02`, `🏃 Deploy: running job x...`, `❌ Test: unit failed · 1:40` with the failed log under it), and an expandable quote for the commits. Emoji only as stage marks.
- **Stop, Retry and Run**: a running pipeline carries `Stop pipeline`, which asks `Yes, stop it` / `Keep running` before canceling; a failed one carries `Retry`, no questions asked; a manual job waiting to start gets `Run <job>`. Needs `gitlab.hooks_token`; anyone in the group may press. Everything else is a link in the text.
- **Push cards**: `@ada pushed to agent (feat/x)`, the commits in one quote as `author: title`, closed by the line counts (`+23, -46 lines on 4 files`), and the pipeline that push triggered, all edited in place. A newer push to the same branch gets its own card. Branch created and deleted, and a force-push warning (detected via the API, because GitLab doesn't tell you).
- **Handles, not pings**: people show as bold `@gitlab-user` that never links to a Telegram account, so a card never notifies anyone.
- **Forum topics**: route pipelines and pushes to their own topics.
- **Reconciler**: cards that stopped receiving events get re-read from the GitLab API. GitLab does not retry failed webhook deliveries, ever, so somebody has to.
- **`sync-hooks`**: registers the webhook on every project in the group, or one group hook if you pay for Premium.
- Everything else is a tap away through the links in the card; there are no link buttons.

## Quick start

### 1. Telegram

1. `/newbot` at [@BotFather](https://t.me/BotFather), keep the token.
2. Add the bot to your group and make it an admin (forum topics need it).
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
| `defaults.events` | all | subset of `push pipeline` |
| `defaults.verbosity` | `normal` | `quiet` (final state only), `normal`, `verbose` (per-job lines, queued time) |
| `defaults.branches.allow` / `.deny` | `["*"]` / `[]` | globs (`release/*`) or `re:` regexps; deny wins |
| `defaults.pipelines.child_cards` | `inline` | `inline` (in parent card), `own`, `both` |
| `defaults.pipelines.quiet_success` | `true` | with `quiet`: stay silent on success |
| `defaults.pipelines.log_tail.lines` | `10` | log lines shown for a failed job; `0` disables them |
| `defaults.push.max_commits` | `10` | commits listed per push |
| `projects[]` | | `path: group/project` plus any `defaults` key and `threads` |
| `users` | | `gitlab_username: telegram_user_id`; unused while cards never mention people |

Events from projects outside `gitlab.group` are ignored unless listed in `projects[]`.

## Webhooks

- **Free tier**: `gitgram sync-hooks` walks every non-archived project under the group and creates or updates a project hook. Re-run it when projects are added. `--dry-run` shows the plan.
- **Premium/Ultimate**: `gitgram sync-hooks --group-hook` registers a single group webhook. Don't do both; every event arrives twice and you get to pay for deduplicating it.

Endpoint: `POST <public_base_url>/webhook/gitlab`. Events to enable: Push, Job, Pipeline. Other events are accepted and ignored.

## How cards behave

- Every pipeline and push owns exactly one Telegram message. Events update stored state; a single sender renders the latest state and edits the message. Identical renders are skipped by hash, so Telegram never sees "message is not modified" and you never see a pointless edit.
- Edits are debounced 700 ms: a burst of job events becomes one edit. First send and final state go out immediately.
- Telegram allows about 20 messages per minute per group, and edits count. The sender rate-limits (1/s, 20/min) and honours `retry_after`. A busy pipeline can delay other messages; it cannot drop them.
- Cards whose message was deleted by a human are marked and left alone. The bot does not resurrect things.
- State and the outbox live in SQLite; restarts resume pending edits, and the reconciler catches up on anything GitLab forgot to deliver.
- Retention: delivery keys 7 days, finished cards 30 days.

## Preview

`gitgram preview --config config.yaml` sends a mock card of every kind and scenario to the configured chat: pushes, branches, a pipeline going pending → running → failed with log tails and artifacts, and a passing and a manual pipeline. Cards go through the real engine and sender, so first sends, in-place edits and folding behave as in production. State lives in a temporary database, nothing touches GitLab, and Telegram is never polled, so it runs beside a live `serve`: `docker compose exec gitgram /gitgram preview --config /config/config.yaml`. Pick scenarios with `--scenario push,pipeline` and pace them with `--delay 4s`. The same thing is one message away in the group: `/preview`, or `/preview push pipeline`.

## Not supported yet

These GitLab events were supported before the card redesign and need cards in the new layout before they come back. Until then the bot ignores them and `sync-hooks` no longer subscribes to them.

- [ ] Merge request cards (opened, draft, merged, closed; approvals, reviewers, threads, conflicts, head pipeline)
- [ ] Merge request comments, replying to the MR card
- [ ] Issue cards (opened, closed, confidential, assignees, labels)
- [ ] Issue comments, replying to the issue card
- [ ] Tag pushes
- [ ] Releases
- [ ] Deployments

## Operations

`GET /healthz` pings SQLite. The container runs read-only as non-root, and `gitgram healthcheck` exists because distroless has no curl. On SIGTERM: stop accepting, drain 30 s, finish the in-flight Telegram call, close the database.

## Roadmap (v2)

- Per-user GitLab tokens linked from a DM, encrypted at rest, so operations run as the person pressing them instead of the bot's token.
- Approve and merge from the MR card.

## License

MIT.
