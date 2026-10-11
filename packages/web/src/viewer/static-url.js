// Anchor navigation to the loaded export, before panels or history change.
export function staticExportURL(documentURL, documentPath, targetPath) {
  const depth = documentPath.split("/").length - 1;
  const root = new URL("../".repeat(depth) || "./", documentURL);
  return new URL(targetPath.split("/").map(encodeURIComponent).join("/"), root).href;
}
