# Homepage product walkthrough

A 33-second, silent walkthrough assembled from actual Open Knowledge viewer
captures. It is an edited sequence of product states, not a continuous screen
recording. The terminal card reproduces the results of a real setup import.
Relay is a fictional webhook project created for this demonstration.

## Static viewer preview

`../public/media/project-wiki-viewer.png` is a real local-viewer screenshot
of `examples/example-project-docs/Wiki`. It shows
`architecture/editorial-workflow.md` with Preview saved page open and the
architecture, conventions, and decisions folders expanded. The homepage and
use-case index share this image. Capture the whole viewer viewport when updating
it; the online demo linked beside it is a read-only HTML export.

## Reproduce the environment

From the repository root, with the CLI installed:

```sh
okn setup /tmp/okn-film/Wiki --from packages/web/demo/sources --mode copy
okn view --name Relay --no-browser --port 4185 /tmp/okn-film/Wiki
```

Use a fresh target directory on each run. Open the printed local viewer URL.
Capture only this demo project; do not include personal registries or documents.

1. Open `imported/overview.md`.
2. Search for `retry` and open `imported/delivery.md`.
3. Add `Honor the Retry-After header on HTTP 429 responses.` after the decision.
4. Inspect Changes, then Save. Confirm the Markdown file contains the change.

The recorded edit was applied through the local viewer's WebMCP editor tool.
The Save action ran through the visible UI. This demonstrates the shared draft;
it does not claim an autonomous agent discovered or verified the retry policy.

## Edit the film

- `sources/`: original demo Markdown.
- `captures/`: actual UI captures and title cards used in the film.
- `title-cards.html`: editable title-card layouts; `?scene=setup|browse|search|edit|end`.
- `render-video.py`: cuts, crops, timing, and H.264 encoding. Requires Python 3 and FFmpeg.
- `../public/media/knowledge-workflow.vtt`: English descriptions, synchronized to the eight scenes.

```sh
python3 packages/web/demo/render-video.py
pnpm build:web
```

Output: `public/media/knowledge-workflow.mp4` (1280 × 800, 33 seconds) and its
JPEG poster. The site uses native video controls, explicit playback, and no
preload. Keep captions and the duration label in sync if the timing changes.
