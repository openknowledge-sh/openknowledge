---
type: Feature Documentation
title: Product Telemetry and Privacy
description: Describes bounded CLI and website telemetry, data limits, and privacy controls.
tags: [openknowledge, telemetry, analytics, privacy]
timestamp: 2026-08-11T00:00:00Z
---

# Product Telemetry and Privacy

Open Knowledge collects bounded product telemetry to measure installation,
activation, useful command activity, daily activity, and sanitized errors.

CLI telemetry is enabled by default. The CLI prints a disclosure before it
sends the first event. Telemetry commands do not send events. JSON error mode
waits for a prior telemetry disclosure. Installer preflight and continuous
integration do not send telemetry.

Google Analytics uses Advanced Consent Mode v2 on the landing and getting-started
pages. Before a visitor chooses, `analytics_storage` is `denied`. Google receives
cookieless measurement pings and does not read or write Analytics cookies.

Select **Allow** to set `analytics_storage` to `granted`. Google Analytics can
then use first-party Analytics cookies. `ad_storage`, `ad_user_data`, and
`ad_personalization` remain `denied` in both states.

The first-party website relay requires explicit consent. It records no website
event before consent.

Use **Analytics preferences** in the website footer to change the website
choice. **No cookies** deletes the random browser ID and Analytics cookies.
Cookieless Google Analytics measurement continues.

## CLI controls

```sh
okn telemetry status
okn telemetry show-payload
okn telemetry disable
okn telemetry enable
okn --no-telemetry <command>
```

Put `--no-telemetry` before the command. This option disables telemetry for the
current command and future commands. It also deletes the random installation ID.

Set `DO_NOT_TRACK=1` or `OPENKNOWLEDGE_TELEMETRY=off` for a process-level
override. These variables do not change the saved preference.

The CLI stores its preference under the operating system user configuration
directory. The file is `openknowledge/telemetry.json` and uses user-only access.

## Collected data

CLI events can include only these fields:

- Random installation and event IDs
- CLI version, operating system, and architecture
- Allowlisted command and subcommand names
- Success, usage error, or command failure
- A coarse duration bucket
- First command, successful setup, first meaningful use, and daily activity

First-party website events can include a random browser ID after consent. They
can record a landing-page view or a successful setup-prompt copy.

Google cookieless pings can include the page URL, user agent, screen resolution,
and IP address. Google states that Google Analytics does not store or log the
IP address. See the
[Consent Mode reference](https://support.google.com/analytics/answer/13802165).

The `/install` redirect records an aggregate install attempt. It stores only a
fixed source label and a normalized client family, such as `curl` or `browser`.
The homepage copy uses a visible fixed `source=homepage` query. It does not use
a browser or installation identifier.

`install_redirect_requested` records a request for the tracked install path. It
does not confirm a completed installation. `cli_first_command` records the first
observed CLI run. Setup and meaningful-use events require successful commands.

## Excluded data

Open Knowledge first-party telemetry envelopes do not contain:

- Command arguments, paths, file names, URLs, queries, or repository names
- Knowledge content, command output, error messages, or agent transcripts
- User accounts, email addresses, hostnames, or machine-derived identifiers
- IP addresses or raw user-agent strings

The hosting platform processes network connections. The first-party relay does
not add request IP addresses or raw user agents to events or upstream requests.

These exclusions do not apply to direct Google Analytics requests. Google
controls its processing under its service terms and configured data controls.

## Delivery

The CLI sends one bounded JSON envelope after a command finishes. Delivery uses
a short timeout. A delivery failure does not change output or exit status.

The first-party relay accepts only documented fields and values. It rejects
extra content. The relay converts accepted envelopes to PostHog's batch capture
format only when an operator configures a PostHog ingestion endpoint and project
token. The project token stays on the server. First-party website and CLI
clients send only to the Open Knowledge relay.

The website also sends Google Analytics requests directly to Google. Consent
Mode attaches the current storage and advertising consent states to those
requests.

Every PostHog event sets `$process_person_profile` to `false`. The relay uses a
surface-prefixed random ID as `distinct_id`: the random installation ID for CLI
events, the consent-created random browser ID for web events, and the random
event ID for aggregate install redirects. These identities are intentionally
not joined across surfaces.

Sanitized failures appear as the `cli_error` product event. The relay also
creates a synthetic PostHog `$exception` event for native issue grouping.

The synthetic exception uses only the command and `error_kind` for its
fingerprint. It uses a fixed description and does not contain an error message,
stack trace, source path, command arguments, or output.

The CLI event schema does not change. The CLI does not use the PostHog Go SDK
or receive a PostHog project token.

Session observation is a separate local feature. `--observe` remains opt-in and
does not change product telemetry.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/telemetry/`
> - `packages/cli/cmd/openknowledge/telemetry_command.go`
> - `packages/web/src/analytics.js`
> - `packages/web/scripts/server.mjs`
>
> **Update notes**
>
> Update this page after a telemetry event, consent, identity, or retention change.
