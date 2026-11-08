日本語: [README.ja.md](README.ja.md)

# switchboard

switchboard is a personal tool that starts unattended Claude Code runs from GitHub and GitLab issues, across several hosts and repositories. When the agent stops on an issue with a question, the issue goes into one inbox as waiting. You can reply from the CLI, a browser, or the Android app.

The project site is at <https://uskanda.github.io/switchboard/>.

## What it solves

- Claude Code sessions are scattered across hosts and repositories, and you cannot tell which ones are waiting for your reply
- You do not want to go through each product's issues to check on them
- You do not want to walk over to a host's terminal to reply

## How it works

1. Put `agent:ready` on an issue (`agent:plan` to start with a plan)
2. The agent on the host in charge of the project runs `claude -p` in a worktree of its own for the issue
3. If a run ends with a question, the agent posts it as an issue comment and the issue becomes `agent:waiting`. Telegram and the Android app notify you of waiting issues
4. Reply from the inbox (`switchboard inbox`, the web inbox, or the app), and the agent resumes the run on the same issue
5. Once the work is done, the agent pushes a branch that passed the verify command, opens a draft PR/MR, and sets `agent:done`. You can merge it from the inbox

When CI fails, a push is rejected, or the branch conflicts with the base, the agent automatically starts a run to fix it.

## Principles

| Principle | What it means |
|---|---|
| Issue-driven | Issues hold the unit of work, its state, and the path of every reply |
| Hosts connect outward only | Hosts open no listening ports. The one exception is the web inbox, and only on LAN addresses (ADR-036) |
| The relay only rings the bell | The relay passes on only the issue ID. The local agent always fetches the issue text from the forge itself |
| Issue text is data | Only comments by `trusted_authors` count as instructions |
| No credentials for `claude` | The agent pushes, opens PRs, and posts comments. A run executes in a sandbox and cannot reach the network |
| The cross-forge list is built locally | The inbox combines the results of label searches on both forges |

## Features

- Plan first: before implementing, the agent comments its questions and plan. It implements after you approve with a `/ok` reply
- Several hosts and projects: each project has one host in charge of it. Two hosts never run the same issue at once
- Inbox: the CLI, the web inbox for your LAN (`switchboard-web`), and the Android app (`app/`). From the web inbox you can create issues, add projects, and merge PRs/MRs
- Notifications: Telegram (sent by the agent) and Android push notifications through FCM (sent by the inbox service)
- Relay (`switchboard relay serve`): receives GitLab webhooks and starts a run within seconds of your reply
- Running as a service: install the agent as a systemd service and pass the tokens as encrypted systemd credentials
- Run control: cap the number of concurrent runs and queue the rest, pause at the usage limit, and cap the API cost. You can also choose the model and effort, and give runs additional instructions and permissions

## Status

The features of Phase 1 through Phase 5 are implemented. They run on the author's host (Ubuntu 24.04) against real GitHub and GitLab (self-hosted CE). Two things are still unchecked: a real setup with two hosts, and the behavior of some forge APIs. The open items are listed under the verification items in [docs/ROADMAP.md](docs/ROADMAP.md) (Japanese).

## Assumptions and limits

switchboard was built and tested on its author's own setup. Before you use it, check that your setup fits the points below.

- It is for one user. Put only yourself in `trusted_authors`. By default, the agent also posts as your own account. Public repositories work too, but then you cannot use `include_untrusted`, and only input from `trusted_authors` counts as instructions (ADR-062)
- Hosts run Linux and need systemd, Secret Service (GNOME Keyring or similar), `bubblewrap`, and `socat`. switchboard has not been tested on macOS
- The web inbox does not authenticate users. Anyone on a device inside your LAN can reply and create issues
- The relay listens on `127.0.0.1`. Webhooks from GitHub.com cannot reach it, so switchboard polls GitHub projects
- On GitHub, only github.com is supported. GitHub Enterprise Server is not
- No binaries or APKs are published. You build switchboard from source, and you build the Android app with your own Firebase project and signing key
- The web inbox shows the first of Japanese and English in the browser's preferred languages, and English when neither is there. In the Display section of the Settings page, you can fix the language to "日本語" or "English". The web inbox keeps this choice in each browser and never sends it to the server. On Android, the app follows the device language. `comments.language` sets the language of the agent's issue comments and Telegram notifications to English or Japanese. When the web inbox posts a reply or a request to run again to an issue, it writes in the language it shows
- switchboard has not checked whether subscription authentication may be used for unattended runs from a resident service. Read Claude's terms of use and decide for yourself

## Requirements

- Linux (tested on Ubuntu 24.04) with systemd
- Go 1.24 or later, and git
- Node.js 24 and pnpm 12 (to build the web inbox)
- Claude Code CLI (`claude`). `switchboard doctor` warns when it is older than the tested version
- Secret Service and `secret-tool` (the `libsecret-tools` package on Ubuntu)
- `bubblewrap` and `socat` (for the run sandbox)
- `gh` v2.99.0 or later, if you attach screenshots of finished work in GitHub projects

## Getting started

Clone the repository, build it, and install it into `~/.local/bin`.

```bash
git clone https://github.com/uskanda/switchboard.git && cd switchboard
```

```bash
make install
```

Create the forge tokens and store them in the Secret Service. [docs/GUIDE.md](docs/GUIDE.md#registering-tokens) lists the permissions each token needs. `secret-tool` reads the token from standard input.

```bash
secret-tool store --label='switchboard github' service switchboard/github username token
```

```bash
secret-tool store --label='switchboard github inbox' service switchboard/github-inbox username token
```

Copy the example config, then replace `YOUR_GITHUB_USER`, the host name, and the project paths.

```bash
mkdir -p ~/.config/switchboard && cp config/switchboard.example.yaml ~/.config/switchboard/config.yaml
```

Check the config, the tokens, and the `claude` version. Then create the state labels in the target repositories.

```bash
switchboard doctor
```

```bash
switchboard labels sync
```

Start the agent and put `agent:ready` on an issue. Find waiting issues in the inbox and reply to them.

```bash
switchboard agent run
```

```bash
switchboard inbox
```

```bash
switchboard reply 1 "Go with option B."
```

[docs/GUIDE.md](docs/GUIDE.md) covers running the agent as a service and setting up the relay, notifications, the web inbox, and the Android app.

## Documentation

| File | Contents |
|---|---|
| [docs/GUIDE.md](docs/GUIDE.md) | Operations guide (tokens, configuration, how to use and set up each feature) |
| [docs/SPEC.md](docs/SPEC.md) (Japanese) | Intended specification (requirements, state model, CLI) |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (Japanese) | Structure, components, and the flow of processing |
| [docs/DESIGN.md](docs/DESIGN.md) (Japanese) | System design (draft ADRs, detailed design, additional verification items) |
| [docs/SECURITY.md](docs/SECURITY.md) | Threat model and countermeasures |
| [docs/DECISIONS.md](docs/DECISIONS.md) (Japanese) | Record of design decisions (ADRs) |
| [docs/ROADMAP.md](docs/ROADMAP.md) (Japanese) | Phased implementation plan and verification items |
| [docs/PRIOR_ART.md](docs/PRIOR_ART.md) (Japanese) | Similar projects and open source tools we looked at |
| [docs/examples/](docs/examples/) (Japanese) | Examples of instructions for runs (OpenSpec, screenshots) |
| [config/switchboard.example.yaml](config/switchboard.example.yaml) | Example configuration file |
| [AGENTS.md](AGENTS.md) (Japanese) | Instructions for developing this repository with Claude Code |

## Development

```bash
make verify
```

`make verify` runs the Go checks and tests, and the checks for `web/` and `app/` as well. Only `make app-apk`, which builds the Android APK, also needs a JDK and the Android SDK. [DESIGN.md](DESIGN.md) (Japanese) sets the visual standard for the screens, and [PRODUCT.md](PRODUCT.md) (Japanese) describes who uses switchboard and why. The project site lives in `site/`, and GitHub Pages publishes it when it reaches main.

A separate test drives a real `claude` through a whole cycle: starting work, asking a question, receiving the reply, and finishing. It calls the model, so `make verify` and CI do not run it.

```bash
go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/integration/
```

In a development environment without a Secret Service, add `secrets: {allow_env: true}` to the config. With this setting, switchboard reads a token from an environment variable when the Secret Service has no entry for it. For a `token_ref` of `switchboard/github`, the variable is `SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB`.

## License

MIT. See [LICENSE](LICENSE).
