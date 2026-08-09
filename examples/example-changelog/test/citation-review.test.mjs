import assert from "node:assert/strict";
import test from "node:test";

import { reviewCitation } from "../src/citation-review.mjs";

test("returns a stable success code", () => {
  assert.deepEqual(
    reviewCitation({
      claim: "The source directly supports this claim.",
      sourceUrl: "https://example.com/source",
    }),
    {
      accepted: true,
      code: "ready-for-editor-review",
      claim: "The source directly supports this claim.",
      sourceUrl: "https://example.com/source",
    },
  );
});

test("reports an invalid URL", () => {
  assert.deepEqual(
    reviewCitation({ claim: "A claim", sourceUrl: "not a URL" }),
    { accepted: false, code: "source-url-invalid" },
  );
});

test("reports a non-HTTPS source", () => {
  assert.deepEqual(
    reviewCitation({ claim: "A claim", sourceUrl: "http://example.com/source" }),
    { accepted: false, code: "source-https-required" },
  );
});
