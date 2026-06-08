# End-User Documentation Audit — v1.0.0

Audit date: 2026-06-08

## Scope

Assessment of documentation available to end users — the people who
use a Fox assistant provisioned by an operator running Fox Fleet.
End users interact with Fox via a browser; they don't interact with
Fleet directly. This audit checks what documentation exists, where
it lives, and what's missing.

---

## Audience distinction

| Role | Interacts with | Docs belong in |
|------|---------------|---------------|
| **Operator** | Fox Fleet (`fox-control` CLI, panel, config) | `fox-fleet` repo |
| **End user** | Fox assistant (browser chat UI at assigned URL) | `fox-in-the-box` repo |

Fleet's README should link to end-user docs but not host them.
The assistant is Fox; Fleet is the management plane.

---

## What exists in `fox-in-the-box`

### README.md

**Status:** Partially end-user focused, primarily installer/setup.

**Covers (end-user relevant):**
- Value proposition ("private AI assistant that runs on your computer")
- Feature list: chat, memory, multi-profile, skills, scheduling,
  remote access, local AI
- How it works (plain-language architecture)
- 6-question FAQ (privacy, coding, API keys, mobile, cost, updates)
- Post-setup guidance (provider switching, local models, Tailscale,
  updating, resetting)
- Supported providers table (15+ options)

**Missing:**
- No "Getting Started" walkthrough for a new user opening the chat
  UI for the first time
- No feature tutorials (how to use skills, manage memory, create
  profiles, use scheduling)
- No troubleshooting section beyond FAQ basics
- No settings reference
- No privacy/data-handling deep-dive

### docs/ directory

**Status:** Developer/operator focused. Zero end-user documentation.

**Contents:**
- `DEV_MODE.md` — local development workflows
- `RELEASE_WORKFLOW.md` — how to cut releases
- `RESET.md` — the only end-user-adjacent document (how to reset /
  clean install, platform-specific)
- `architecture/` — 17 internal planning files (PRODUCTS.md,
  ENTERPRISE_ARCHITECTURE.md, INSTANCE_CONTRACT.md, ADRs, etc.)
- `archive/` — deprecated strategy and task docs
- `explorations/` — DigitalOcean Marketplace spec
- `ops/` — APT repository setup
- `plans/` — bare-metal .deb packaging
- `specs/` — DigitalOcean Droplet 1-Click spec

### In-app help

**Status:** Minimal, onboarding-focused.

- Onboarding wizard: provider selection, API key entry, Ollama
  detection, model download — handles first-time setup
- Settings dialogs: placeholder copy, labeled controls
- Inline error messages: actionable text for Docker/WSL failures
- No "Help" menu item or link to external documentation
- No contextual tooltips on advanced features
- No in-app tutorial or guided tour

### CHANGELOG.md

**Status:** Detailed per-version release notes (171 KB, 3,309 lines).
Features described in plain language. Not structured as user
documentation — serves as a release log, not a reference.

---

## What's missing for end users

### Tier 1 — high impact, needed for v1.0.0 "usable" claim

| # | Gap | Description | Belongs in |
|---|-----|-------------|-----------|
| EU-01 | Getting Started guide | "You've opened your Fox URL. Here's what you see, here's your first chat." | fox-in-the-box |
| EU-02 | Feature reference | One section per major feature: chat basics, slash commands, memory, profiles, skills, scheduling, workspace | fox-in-the-box |
| EU-03 | Troubleshooting | "Chat won't load," "provider key errors," "Fox seems slow," "no models available" | fox-in-the-box |
| EU-04 | Settings reference | Complete list of user-controllable settings, defaults, what each does | fox-in-the-box |

### Tier 2 — medium impact

| # | Gap | Description | Belongs in |
|---|-----|-------------|-----------|
| EU-05 | Privacy deep-dive | Data flow diagram, what leaves the device, encryption, retention, deletion | fox-in-the-box |
| EU-06 | Slash command reference | 26 commands, what each does, syntax, examples | fox-in-the-box |
| EU-07 | Keyboard shortcuts | If any exist — document them | fox-in-the-box |
| EU-08 | Performance tips | When to compress sessions, manage memory, choose models for speed vs quality | fox-in-the-box |

### Tier 3 — nice to have

| # | Gap | Description | Belongs in |
|---|-----|-------------|-----------|
| EU-09 | In-app Help link | Menu item or button linking to external docs | fox-in-the-box |
| EU-10 | Video tutorials | Walk-throughs for visual learners | fox-in-the-box |

---

## What exists in `fox-fleet` for end users

**Nothing.** Fleet's README is entirely operator/developer focused.
No section tells an operator "here's what to send your employees."

### What's needed in `fox-fleet`

| # | Gap | Description |
|---|-----|-------------|
| FL-01 | "What end users see" section in README | Brief intro to the Fox experience + link to fox-in-the-box end-user docs |
| FL-02 | Operator guidance on employee onboarding | How to share the instance URL, what to tell employees about API keys (if they need their own), what the onboarding wizard handles |

---

## Recommendation

**Option A (recommended):** End-user documentation lives in
`fox-in-the-box`. Fox is the assistant; the end user interacts with
Fox, not Fleet. Fleet's README links there. This matches the
dependency direction (Fleet wraps Fox, not the reverse).

**What to do now:**

1. File EU-01 through EU-04 as issues in `fox-in-the-box` — these
   are the high-impact gaps.
2. Add FL-01 and FL-02 to Fleet's README in this PR — these are
   cross-links and operator guidance.
3. EU-05 through EU-10 go into the fox-in-the-box backlog for v0.8+.

**What NOT to do:** Don't duplicate Fox's end-user docs inside Fleet.
Operators need to know where to point employees, not to maintain a
second copy of "how to use Fox."

---

## Current state of the Fox assistant UI

For context on what end users actually see (relevant to EU-01):

- **Chat panel** — messages, tool-call progress, markdown, syntax
  highlighting, streaming responses
- **Session sidebar** — session list, search, projects, tags, archive
- **Workspace browser** — directory tree, inline file preview,
  edit/create/delete
- **Composer footer** — model picker, workspace selector, profile
  selector, context indicator
- **Control Center** — conversation management (export/import/clear),
  preferences (theme, send key, toggles), system info (version,
  password, Tailscale status)
- **Slash commands** — 26 commands including `/new`, `/clear`,
  `/compress`, `/branch`, `/queue`, `/interrupt`, `/steer`, `/voice`,
  `/skills`, `/personality`, `/usage`
- **Onboarding wizard** — provider selection, API key entry, Ollama
  auto-detection, model download

The UI is self-discoverable for basic chat. Advanced features (skills,
scheduling, branching, workspace) have no external documentation
and rely on users finding them through the UI.

---

## Action items

| # | Action | Target repo | Priority |
|---|--------|-------------|----------|
| 1 | File EU-01..04 as issues in fox-in-the-box | fox-in-the-box | High |
| 2 | Add "What end users see" section to Fleet README | fox-fleet (this PR) | High |
| 3 | Add operator employee-onboarding guidance | fox-fleet (this PR) | Medium |
| 4 | File EU-05..08 as backlog issues in fox-in-the-box | fox-in-the-box | Low |
| 5 | Consider in-app Help link (EU-09) for v0.8+ | fox-in-the-box | Low |
