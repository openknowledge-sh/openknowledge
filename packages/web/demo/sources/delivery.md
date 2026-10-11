# Why we retry webhooks

## Decision

Retry failed deliveries for 24 hours with exponential backoff.

A receiver can be unavailable during a deploy. Retrying lets it recover without losing an event.

## Delivery guarantee

Delivery is at least once. Each event carries a stable event ID. Receivers must deduplicate by that ID.

## What we retry

- Network timeouts.
- HTTP 429 responses.
- HTTP 5xx responses.

## What needs a person

A failed delivery after 24 hours moves to the dead-letter queue. Support can inspect the event and replay it.

## Related notes

- [Project overview](overview.md)
- [Operations](operations.md)
