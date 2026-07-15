# Open Knowledge storage schemas v1

These Draft 2020-12 schemas describe current CLI-owned local persistence:

* `registry.schema.json` covers the path-keyed user registry.
* `cache-source.schema.json` covers the owner-only provenance sidecar stored
  beside each managed cache generation.
* `source.schema.json` is their shared managed-source definition.

The website publishes the directory at
`https://openknowledge.sh/schemas/cli/storage/v1/`. Storage versioning is
independent of the CLI machine-output schema, OKF spec, and portable manifest
protocol versions.

The runtime accepts a missing registry `schemaVersion` only as a legacy v0
migration path. Every new registry write emits version `1`; unsupported
versions, unknown fields, duplicate object keys, trailing JSON, invalid paths,
keys, access values, and ambiguous duplicate logical keys fail closed.
Registry reads are capped at 8 MiB and individual cache provenance sidecars at
1 MiB before decoding.
