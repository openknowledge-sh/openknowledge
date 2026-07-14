package okf

// MachineSchemaVersion identifies the JSON contract shared by CLI machine
// outputs. Additive fields remain within the same major version; incompatible
// shape or semantic changes require a new version and schema set.
const MachineSchemaVersion = "1"
