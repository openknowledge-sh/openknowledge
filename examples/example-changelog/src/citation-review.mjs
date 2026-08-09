export function reviewCitation({ claim, sourceUrl }) {
  const normalizedClaim = typeof claim === "string" ? claim.trim() : "";
  if (!normalizedClaim) {
    return { accepted: false, code: "claim-required" };
  }

  let parsedSource;
  try {
    parsedSource = new URL(sourceUrl);
  } catch {
    return { accepted: false, code: "source-url-invalid" };
  }

  if (parsedSource.protocol !== "https:") {
    return { accepted: false, code: "source-https-required" };
  }

  return {
    accepted: true,
    code: "ready-for-editor-review",
    claim: normalizedClaim,
    sourceUrl: parsedSource.href,
  };
}
