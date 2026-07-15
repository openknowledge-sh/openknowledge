// Package okf exposes the read-only Open Knowledge Format core used by the
// Open Knowledge CLI. It lets Go applications parse, validate, inspect,
// search, resolve read-only registry connections and capabilities, and select
// source-grounded context from local OKF bundles without spawning the CLI or
// decoding command output.
//
// Functions without an explicit version use LatestSpecVersion. Integrations
// that persist results should prefer the WithVersion forms and record the
// returned SpecVersion and SchemaVersion fields.
package okf
