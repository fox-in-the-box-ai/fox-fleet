# Fox Fleet

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Open-source management plane for [Fox in the Box](https://github.com/fox-in-the-box-ai/fox-in-the-box). Provision and manage a fleet of Fox AI assistants on a single Docker host through a browser-based panel.

**Status:** v0.1 in development. Not yet usable.

## What Fox Fleet does

Fox Fleet wraps Fox instances the same way the Fox overlay wraps Hermes: additive behavior, fully removable, fail-loud on errors. An operator runs `fox-control serve`, opens the panel, and manages assistants without touching Docker directly. Every instance is an unmodified Fox container — Fleet adds management, not modification.

## Quickstart

Coming in v0.1. See the [Fleet Base Roadmap](https://github.com/fox-in-the-box-ai/fox-in-the-box/blob/main/docs/architecture/FLEET_BASE_ROADMAP.md) for timeline.

## Repo layout

```
cmd/fox-control/     CLI + API server entry point
plugins/             DeploymentPlugin interface + Docker implementation
panel/               Dashboard API + embedded SPA
conformance/         Runtime + plugin conformance test suites
data-plane/          Shared Qdrant + ingestion + query API (v0.2)
skillsets/           Skillset manifest spec + Hermes adapter (v0.2)
rollout/             Fleet rollout orchestration (CLI)
internal/            Registry, config injection, provisioner
```

## Architecture

Design docs live in the Fox repo: [docs/architecture/](https://github.com/fox-in-the-box-ai/fox-in-the-box/tree/main/docs/architecture). Key references:

- [DEMO_TIER.md](https://github.com/fox-in-the-box-ai/fox-in-the-box/blob/main/docs/architecture/DEMO_TIER.md) — Fleet v0.1 MVP spec
- [FLEET_BASE_V01_SPEC.md](https://github.com/fox-in-the-box-ai/fox-in-the-box/blob/main/docs/architecture/FLEET_BASE_V01_SPEC.md) — implementation-ready ticket specs
- [PRODUCTS.md](https://github.com/fox-in-the-box-ai/fox-in-the-box/blob/main/docs/architecture/PRODUCTS.md) — three-product open-core split

## License

Apache License 2.0. See [LICENSE](LICENSE).
