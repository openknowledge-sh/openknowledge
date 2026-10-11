"""Render the homepage walkthrough from real viewer captures. Requires ffmpeg."""
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent
CAPTURES = ROOT / 'captures'
OUTPUT = ROOT.parent / 'public' / 'media'
OUTPUT.mkdir(parents=True, exist_ok=True)


def ffmpeg(*args):
    subprocess.run(['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y', *map(str, args)], check=True)


# Crops focus on the actual search result, draft, diff, and persisted state.
SCENES = [
    ('01-setup', 5, None, None),
    ('02-browse', 4, 'browse', None),
    ('03-search', 4, 'search', 'crop=1260:708:420:0'),
    ('04-decision', 5, 'search', 'crop=900:506:650:55'),
    ('05-edit', 4, 'edit', 'crop=900:506:650:55'),
    ('06-diff', 4, 'edit', 'crop=900:506:650:450'),
    ('07-saved', 4, 'edit', 'crop=900:506:650:55'),
    ('08-end', 3, None, None),
]

with tempfile.TemporaryDirectory(prefix='okn-film-') as scratch:
    scratch = Path(scratch)
    clips = []
    for name, duration, header, crop in SCENES:
        still = scratch / f'{name}.png'
        source = CAPTURES / f'{name}.png'
        if header:
            view_filter = (crop + ',' if crop else '') + 'scale=1120:630'
            ffmpeg('-i', CAPTURES / f'header-{header}.png', '-i', source,
                   '-filter_complex', f'[0:v]scale=1280:720,pad=1280:800:0:0:color=0x080b0d[base];[1:v]{view_filter}[screen];[base][screen]overlay=80:142',
                   '-frames:v', 1, still)
        else:
            ffmpeg('-i', source, '-vf', 'scale=1280:720,pad=1280:800:0:0:color=0x080b0d', '-frames:v', 1, still)
        if name == '02-browse':
            ffmpeg('-i', still, '-q:v', 2, OUTPUT / 'knowledge-workflow-poster.jpg')
        clip = scratch / f'{name}.mp4'
        ffmpeg('-loop', 1, '-framerate', 30, '-i', still, '-t', duration,
               '-vf', f'fade=t=in:st=0:d=0.18,fade=t=out:st={duration - .18}:d=0.18,format=yuv420p',
               '-c:v', 'libx264', '-preset', 'fast', '-crf', 19, '-an', clip)
        clips.append(clip)
    manifest = scratch / 'concat.txt'
    manifest.write_text(''.join(f"file '{p}'\n" for p in clips))
    ffmpeg('-f', 'concat', '-safe', 0, '-i', manifest, '-c', 'copy', '-movflags', '+faststart', OUTPUT / 'knowledge-workflow.mp4')
print(OUTPUT / 'knowledge-workflow.mp4')
