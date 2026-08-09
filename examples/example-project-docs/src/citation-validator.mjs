export function validateCitation({ claim, sourceUrl }) {
  const normalizedClaim = typeof claim === "string" ? claim.trim() : "";
  if (!normalizedClaim) {
    return { valid: false, code: "claim-required" };
  }

  let parsedSource;
  try {
    parsedSource = new URL(sourceUrl);
  } catch {
    return { valid: false, code: "source-url-invalid" };
  }

  if (parsedSource.protocol !== "https:") {
    return { valid: false, code: "source-https-required" };
  }

  return {
    valid: true,
    claim: normalizedClaim,
    sourceUrl: parsedSource.href,
  };
}
