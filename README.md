# kar — کار

**kar** (Persian for *work*, *deed*) is a tiny Go CLI that keeps Atlassian
**Jira** work items and **Jira Product Discovery** roadmaps up to date from
the terminal — built to be driven by humans, scripts, and AI agents alike.

```
$ kar stage v0.4 Delivery
HAMROADMAP-11 (v0.4 Calendar & Files) -> Delivery

$ kar done HAMN-87 --comment "merged via MR !42"
HAMN-87 -> Done

$ kar sync
DRIFT  HAMROADMAP-11  v0.4 Calendar & Files      HAMN-43 is Done, idea is "Delivery"
info   HAMROADMAP-15  v0.8 Agent Maintainability Project start or Project target not set
8 idea(s) checked, 1 stage drift(s)
$ echo $?
2
```

## Why

Roadmaps rot because updating them costs a step nobody takes. `kar` makes
that step one command: flip the work item, move the idea, set the dates,
run `sync` to prove the two sides agree. It exists to encode the sharp
edges of the Jira + Product Discovery API pair — workflow transitions,
interval-typed date fields, idea-key resolution — into boring, testable
verbs instead of tribal knowledge.

## Install

Requires Go 1.22+.

```
go install github.com/sonemaro/kar@latest
# or from a clone:
make build   # -> bin/kar
```

## Configuration

`kar` needs three things: your site, your email, and an API token
(create one at id.atlassian.com → Security → API tokens). Supply them via
flags, environment, or a config file — later sources override earlier
ones (flags > env > file).

Environment: `KAR_SITE`, `KAR_EMAIL`, `KAR_TOKEN` (or `JIRA_API_TOKEN`),
`KAR_CONFIG`.

`~/.config/kar/config.json`:

```json
{
  "site": "https://your-site.atlassian.net",
  "email": "you@example.com",
  "token_file": "~/.jira-token",
  "work_project": "HAMN",
  "roadmap_project": "HAMROADMAP",
  "field_project_start": "customfield_10059",
  "field_project_target": "customfield_10053",
  "field_roadmap_lane": "customfield_10054"
}
```

`work_project` is your delivery/execution project; `roadmap_project` is
the Product Discovery project. The three `field_*` ids are the JPD
fields `kar` writes: **Project start**, **Project target** (the two
interval fields JPD timelines draw bars from) and the roadmap **lane**
select (Now / Next / Later / Won't do). Run `kar doctor` to verify all
of it against your site before using anything else.

Tokens read from a file should be mode 600 — `kar` warns otherwise and
never prints the token.

## Commands

| Command | Effect |
|---|---|
| `kar doctor` | Verify auth, both projects, and the configured fields. |
| `kar find <query> [--project KEY]` | JQL summary search; default project is the work project. |
| `kar done <key> [--comment text]` | Transition to Done (optional comment first). |
| `kar comment <key> <text>` | Add a comment (plain text; blank line = new paragraph). |
| `kar link <keyA> <keyB>` | Create a `Relates` link. |
| `kar stage <idea> <stage>` | Move an idea through its workflow (e.g. Discovery → Delivery). |
| `kar dates <idea> [--start D] [--target D]` | Set Project start / Project target (YYYY-MM-DD). |
| `kar lane <idea> <Now\|Next\|Later\|Won't do>` | Set the roadmap lane field. |
| `kar sync [--fix]` | Reconcile ideas against linked work issues; `--fix` applies the safe fixes. |

Idea arguments accept an issue key (`HAMROADMAP-11`) *or* a summary
fragment (`v0.4`) — a fragment must match exactly one idea.

`kar sync` compares each idea with the work issue it is linked to:
work issue `Done` + idea not `Done` is **stage drift** (fixable), the
reverse is reported as a warning, and missing timeline dates are listed
as information. Exit codes: `0` clean, `2` drift found (without `--fix`),
`1` error — so `kar sync` is safe to use as a scripted check.

Every command takes `--json` for stable machine-readable output.

## Agent-friendly by construction

`kar` is designed to be operated by coding agents with shell access:

- **Deterministic output** — plain tables for humans, `--json` for machines;
  no colors, no spinners, no interactive prompts.
- **Meaningful exit codes** — `0/1/2` as above; `sync` is a check, not just
  an action.
- **Dry by default where it matters** — `sync` reports first, mutates only
  with `--fix`; every mutation prints exactly what it did.
- **One-line mental model** — `kar doctor` first, `kar sync` to see, the
  verbs to change.

A minimal agent loop:

```
kar doctor && kar sync                 # understand current state (sync exits 2 on drift)
kar done HAMN-87 --comment "MR !42"    # close the work item at merge time
kar stage v0.4 Delivery                # keep the roadmap honest
kar sync --fix                         # reconcile everything else
```

If you wire an MCP server on top, make each tool wrap one `kar` verb —
the CLI stays the single place that knows the API.

## Licensing & Atlassian terms

- `kar` uses **only the documented Jira Cloud REST API v3** under your own
  user API token. It does not use undocumented endpoints, does not scrape,
  and does not redistribute any Atlassian code or content.
- Usage stays within Atlassian's published rate-limit guidance: `kar`
  self-throttles (default 4 requests/second, `--rps`), honors `Retry-After`
  on HTTP 429, and retries transient 5xx with backoff.
- `kar` is an independent utility and is **not affiliated with, endorsed
  by, or sponsored by Atlassian**. Jira, Jira Product Discovery, and
  Atlassian are trademarks of Atlassian; references here are nominative,
  to describe compatibility only.
- Your API token never leaves your machine except to your own Atlassian
  site over HTTPS. Keep token files mode `600`.

## Development

```
make test   # unit tests
make vet    # go vet
make fmt    # gofmt
```

Dependencies: none — Go standard library only. The entire API surface
lives in `client.go`; adding a verb means one function in a `cmd_*.go`
file and one line in `main.go`.

## Roadmap

- Optional MCP server wrapping each verb as a tool (the CLI stays the
  single source of truth).

## License

[MIT](LICENSE)
