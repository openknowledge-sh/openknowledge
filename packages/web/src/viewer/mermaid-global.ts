import mermaid from "mermaid";

declare global {
  interface Window {
    mermaid: typeof mermaid;
  }
}

window.mermaid = mermaid;
