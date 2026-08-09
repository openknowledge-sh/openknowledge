import assert from "node:assert/strict";
import test from "node:test";

import { validateCitation } from "../src/citation-validator.mjs";

test("accepts a claim with an HTTPS source", () => {
  assert.deepEqual(
    validateCitation({
      claim: "  The source directly supports this claim.  ",
      sourceUrl: "https://example.com/source",
    }),
    {
      valid: true,
      claim: "The source directly supports this claim.",
      sourceUrl: "https://example.com/source",
    },
  );
});

test("requires a claim", () => {
  assert.deepEqual(
    validateCitation({ claim: " ", sourceUrl: "https://example.com/source" }),
    { valid: false, code: "claim-required" },
  );
});

test("requires an HTTPS source URL", () => {
  assert.deepEqual(
    validateCitation({ claim: "A claim", sourceUrl: "http://example.com/source" }),
    { valid: false, code: "source-https-required" },
  );
});
