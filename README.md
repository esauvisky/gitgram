<p align="center">
  <img src="assets/logo.png" width="160" alt="Gitgram logo: a git graph merging into a paper plane">
</p>

<h1 align="center">Gitgram</h1>

<p align="center">GitLab webhooks → one Telegram group. Cards that edit themselves instead of forty messages nobody reads.</p>

---

Gitgram is a self-hosted bot that takes every project under one gitlab.com group and turns its webhooks into Telegram messages that stay useful. A pipeline is **one message** that updates as jobs run, and a push is **one message** that carries its commits and the pipeline it triggered, so the group chat reads like a timeline instead of a log file. Today it covers pushes, pipelines and merge requests; the other GitLab events are listed under [Not supported yet](#not-supported-yet).

It exists because [Integram](https://github.com/requilence/integram) was archived, GitLab's built-in Telegram integration sends one flat line per event and calls it a day, and every other bot on GitHub either stopped in 2022 or only does DMs.

Single static Go binary. SQLite. No cgo. Runs happily in a 20 MB distroless container.

## What you get

- **Pipeline cards**: pending → running → done, edited in place. Stage rows, job names, failed jobs with log links, durations, "waiting for manual job" states, child pipelines folded into the parent. Retries don't spawn a new card, they update the old one.
- **Failure logs**: when a job fails, the last 10 lines of its log appear under its stage, syntax-coloured. Running and passed jobs show no log, just the stage line and, once finished, how long it took. Needs `gitlab.read_token`.
- **Artifacts**: every archive a stage's jobs produced, linked with its size at the end of the stage's line, once the pipeline ends.
- **One layout for every card**, in plain Telegram HTML that any client renders: a title that never changes once posted, `@someone did this in repo (branch)` (`@ada pushed to agent (feat/x)`, `@emi ran pipeline #84 in agent (main)`), italic detail lines, one line per pipeline stage (`✅ Build · 1:02`, `🏃 Deploy: running job x...`, `❌ Test: unit failed · 1:40` with the failed log under it), and an expandable quote for the commits. Emoji only as stage marks.
- **Stop, Retry, Run and Merge**: a running pipeline carries `Stop pipeline`, which asks `Yes, stop it` / `Keep running` before canceling; a failed one carries `Retry`, no questions asked; a manual job waiting to start gets `Run <job>`. Needs `gitlab.hooks_token`; anyone in the group may press. Everything else is a link in the text.
- **Merge request cards**: `@ada opened MR !42 in demo (feat/x → develop)`, the MR title in bold, the description and line counts in a fold, unresolved threads, the MR's head pipeline with its stage lines, and last, in bold, `Merged into develop by @linus`, `Closed by @ada`, or while open `Draft` and `⚠️ Conflicts with develop`. A merge request pipeline shows only there. `Merge` (with a confirmation) appears while GitLab reports the MR as mergeable.
- **Push cards**: `@ada pushed to agent (feat/x)`, the commits in one quote as `author: title`, closed by the line counts (`+23, -46 lines on 4 files`), and the pipeline that push triggered, all edited in place. A newer push to the same branch gets its own card. Branch created and deleted, and a force-push warning (detected via the API, because GitLab doesn't tell you).
- **Handles, not pings**: people show as bold `@gitlab-user` that never links to a Telegram account, so a card never notifies anyone.
- **Reconciler**: cards that stopped receiving events get re-read from the GitLab API. GitLab does not retry failed webhook deliveries, ever, so somebody has to.
- **`sync-hooks`**: registers the webhook on every project in the group, or one group hook if you pay for Premium.
- Everything else is a tap away through the links in the card; there are no link buttons.

## Quick start

### 1. Telegram

1. `/newbot` at [@BotFather](https://t.me/BotFather), keep the token.
2. Add the bot to your group and make it an admin.
3. Get the chat id (`-100…`): forward a group message to [@getidsbot](https://t.me/getidsbot), or open the group in [web.telegram.org](https://web.telegram.org/a/) and read the URL fragment.

### 2. GitLab

| Token | Scope | Used for | Required |
|---|---|---|---|
| `GITGRAM_GITLAB_TOKEN` | `read_api` | diff stats, failure logs, artifacts, force-push detection, reconciler | no, features degrade gracefully |
| `GITGRAM_GITLAB_HOOKS_TOKEN` | `api` | `gitgram sync-hooks` and the Stop, Retry and Run buttons | only for sync-hooks and buttons |
| `GITGRAM_WEBHOOK_SECRET` | any string | `X-Gitlab-Token` on each delivery | yes |

One token with `api` covers both. A group access token works too.

### 3. Run

Prebuilt multi-arch images (amd64, arm64) live at `ghcr.io/esauvisky/gitgram`. No clone needed:

```sh
mkdir gitgram && cd gitgram
curl -fsSLO https://raw.githubusercontent.com/esauvisky/gitgram/main/docker-compose.yml
curl -fsSL  https://raw.githubusercontent.com/esauvisky/gitgram/main/.env.example -o .env
# edit .env: token, chat id, group, public URL, secrets (openssl rand -hex 24)
docker compose up -d
docker compose exec gitgram /gitgram sync-hooks --dry-run
docker compose exec gitgram /gitgram sync-hooks
```

Put a TLS-terminating reverse proxy in front of `127.0.0.1:8080` and set `GITGRAM_PUBLIC_URL` to whatever GitLab can reach. With `GITGRAM_TELEGRAM_MODE=polling` Telegram needs no public URL at all; GitLab still does.

Pin a version with `VERSION=0.1.0 docker compose up -d`. To build locally instead of pulling: clone the repo and `docker compose up -d --build`. Without Docker: `set -a; . ./.env; set +a; go run ./cmd/gitgram serve --poll`, with `GITGRAM_DB` pointing somewhere writable.

## Configuration

Everything comes from `GITGRAM_*` environment variables; under Docker Compose that is the `.env` file next to `docker-compose.yml` (start from `.env.example`). Every problem is reported in one go, not one per restart.

| Variable | Default | Meaning |
|---|---|---|
| `GITGRAM_TELEGRAM_TOKEN` | required | bot token |
| `GITGRAM_CHAT_ID` | required | target group (`-100…`), or several separated by commas; every card goes to each and is edited in each. The first is the primary, the one the bot's bookkeeping follows |
| `GITGRAM_DEBUG_CHAT_ID` | | chat that receives `gitgram preview` mock cards instead of `GITGRAM_CHAT_ID`; `/preview` works there too |
| `GITGRAM_TELEGRAM_MODE` | `webhook` | `webhook` or `polling` |
| `GITGRAM_TG_WEBHOOK_SECRET` | required in webhook mode | URL suffix and `secret_token` for Telegram updates |
| `GITGRAM_PUBLIC_URL` | required for webhook mode and sync-hooks | external base URL; GitLab delivers to `/webhook/gitlab`, Telegram to `/webhook/telegram/<secret>` |
| `GITGRAM_LISTEN` | `:8080` | listen address |
| `GITGRAM_GITLAB_URL` | `https://gitlab.com` | instance URL |
| `GITGRAM_GITLAB_GROUP` | required | top-level group; subgroups included, other projects ignored |
| `GITGRAM_WEBHOOK_SECRET` | required | `X-Gitlab-Token` value |
| `GITGRAM_GITLAB_TOKEN` | | `read_api` token |
| `GITGRAM_GITLAB_HOOKS_TOKEN` | | `api` token for sync-hooks and the buttons |
| `GITGRAM_LOG_LINES` | `10` | log lines shown for a failed job; `0` disables them |
| `GITGRAM_MAX_COMMITS` | `10` | commits listed per push |
| `GITGRAM_DB` | `/data/gitgram.db` | SQLite file |
| `GITGRAM_LOG_LEVEL` / `GITGRAM_LOG_FORMAT` | `info` / `text` | slog level; `text` or `json` |

## Webhooks

- **Free tier**: `gitgram sync-hooks` walks every non-archived project under the group and creates or updates a project hook. Re-run it when projects are added. `--dry-run` shows the plan.
- **Premium/Ultimate**: `gitgram sync-hooks --group-hook` registers a single group webhook. Don't do both; every event arrives twice and you get to pay for deduplicating it.

Endpoint: `POST <GITGRAM_PUBLIC_URL>/webhook/gitlab`. Events to enable: Push, Job, Pipeline, Merge request. Other events are accepted and ignored.

## How cards behave

- Every pipeline, merge request and push owns exactly one Telegram message. Events update stored state; a single sender renders the latest state and edits the message. Identical renders are skipped by hash, so Telegram never sees "message is not modified" and you never see a pointless edit.
- Edits are debounced 700 ms: a burst of job events becomes one edit. First send and final state go out immediately.
- Telegram allows about 20 messages per minute per group, and edits count. The sender rate-limits (1/s, 20/min) and honours `retry_after`. A busy pipeline can delay other messages; it cannot drop them.
- Cards whose message was deleted by a human are marked and left alone. The bot does not resurrect things.
- State and the outbox live in SQLite; restarts resume pending edits, and the reconciler catches up on anything GitLab forgot to deliver.
- Retention: delivery keys 7 days, finished cards 30 days.

## Preview

`gitgram preview` sends a mock card of every kind and scenario to `GITGRAM_DEBUG_CHAT_ID` (or, without one, to every `GITGRAM_CHAT_ID` chat): pushes, branches, a pipeline going pending → running → failed with log tails and artifacts, a passing and a manual pipeline, and a merge request opened → merged plus a closed draft. Cards go through the real engine and sender, so first sends, in-place edits and folding behave as in production. State lives in a temporary database, nothing touches GitLab, and Telegram is never polled, so it runs beside a live `serve`: `docker compose exec gitgram /gitgram preview`. Pick scenarios with `--scenario push,pipeline` and pace them with `--delay 4s`. The same thing is one message away in the group: `/preview`, or `/preview push pipeline`.

## Not supported yet

These GitLab events were supported before the card redesign and need cards in the new layout before they come back. Until then the bot ignores them and `sync-hooks` no longer subscribes to them.

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
