import { describe, expect, it } from "vitest";
import { staticExportURL } from "./static-url.js";

describe("static export navigation", () => {
  it("keeps nested destinations under a hosted export root", () => {
    expect(staticExportURL("https://example.test/demo/", "index.html", "research/finding.html"))
      .toBe("https://example.test/demo/research/finding.html");
  });

  it("resolves sibling and root notes from a directly loaded nested page", () => {
    const loaded = "https://example.test/demo/research/finding.html?stack=index.md";
    expect(staticExportURL(loaded, "research/finding.html", "sources/note.html"))
      .toBe("https://example.test/demo/sources/note.html");
    expect(staticExportURL(loaded, "research/finding.html", "index.html"))
      .toBe("https://example.test/demo/index.html");
  });

  it("supports file exports and escaped note names", () => {
    expect(staticExportURL("file:///tmp/demo/research/finding.html", "research/finding.html", "sources/A #1.html"))
      .toBe("file:///tmp/demo/sources/A%20%231.html");
  });
});
