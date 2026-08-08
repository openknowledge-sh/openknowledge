---
name: Open Knowledge Web
description: A bright project workspace for source-backed knowledge.
colors:
  ink: "#082b63"
  ink-strong: "#061d42"
  text-muted: "#31577f"
  text-quiet: "#526f8d"
  action: "#0a397f"
  action-hover: "#061f4a"
  paper: "#f7f9fb"
  blue-field: "#e5f0f9"
  blue-panel: "#dceaf5"
  terminal: "#071c3d"
  success: "#075a49"
typography:
  display:
    fontFamily: "Manrope, ui-sans-serif, sans-serif"
    fontSize: "clamp(3.55rem, 5.8vw, 5.45rem)"
    fontWeight: 620
    lineHeight: 0.94
    letterSpacing: "-0.04em"
  headline:
    fontFamily: "Manrope, ui-sans-serif, sans-serif"
    fontSize: "clamp(2.45rem, 4.8vw, 4.4rem)"
    fontWeight: 620
    lineHeight: 0.98
    letterSpacing: "-0.04em"
  body:
    fontFamily: "Manrope, ui-sans-serif, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.7
    letterSpacing: "-0.012em"
  mono:
    fontFamily: "IBM Plex Mono, ui-monospace, monospace"
    fontSize: "0.72rem"
    fontWeight: 500
    lineHeight: 1.55
rounded:
  control: "12px"
  panel: "16px"
  navigation: "18px"
components:
  button-primary:
    backgroundColor: "{colors.action}"
    textColor: "{colors.paper}"
    rounded: "{rounded.control}"
    padding: "0 22px"
    height: "50px"
  button-primary-hover:
    backgroundColor: "{colors.action-hover}"
    textColor: "{colors.paper}"
    rounded: "{rounded.control}"
  topbar:
    backgroundColor: "{colors.blue-field}"
    textColor: "{colors.ink}"
    rounded: "{rounded.navigation}"
    height: "64px"
---

# Design System: Open Knowledge Web

## Overview

**Creative North Star: "The Open Project Desk"**

The website feels like a bright workspace where project files, documentation,
and tools remain visible together. Large direct typography establishes the
idea. Real product surfaces and precise source-file geometry provide proof.

The system stays calm and technical without becoming sterile. Light-blue
fields carry page-scale structure. Deep blue gives text and actions one clear
voice.

**Key Characteristics:**

- Large Manrope headlines with compact line height.
- Light-blue page fields and deep-blue text.
- Real viewer captures, terminal surfaces, and authored source diagrams.
- Generous editorial spacing with restrained controls.

## Colors

Deep blue carries language and action. Cool paper and blue fields separate
reading, demonstration, and navigation surfaces.

### Primary

- **Knowledge Blue:** Primary text, navigation, and diagram structure.
- **Action Blue:** Primary actions and important interactive states.

### Neutral

- **Cool Paper:** Default reading surface.
- **Open Sky:** Large explanatory fields and hero regions.
- **Source Panel:** File trees, viewer chrome, and secondary documentation UI.
- **Terminal Navy:** Code, terminal output, and high-contrast source examples.

**The Blue Field Rule.** Use blue as a page-scale field or a clear action. Do
not scatter blue as unrelated decoration.

## Typography

**Display Font:** Manrope with a UI sans-serif fallback
**Body Font:** Manrope with a UI sans-serif fallback
**Label/Mono Font:** IBM Plex Mono with a UI monospace fallback

**Character:** Manrope keeps the site direct and readable at large scale. IBM
Plex Mono appears only where the interface presents files, commands, or data.

### Hierarchy

- **Display:** Use for the one promise that owns the first viewport.
- **Headline:** Use for section conclusions, with a compact measure.
- **Body:** Keep explanatory copy between 58 and 65 characters per line.
- **Label:** Use monospace for paths, versions, commands, and source metadata.

**The One Large Thought Rule.** Let one headline dominate each major field.
Do not compete with parallel headings at the same scale.

## Layout

Use a centered content width of 1120 pixels. Full-width color fields extend
from that content frame to the viewport edges. Desktop sections use unequal
two-column grids so that text and proof do not feel interchangeable.

At tablet widths, reduce the gap before stacking. At 860 pixels and below,
stack narrative sections into one column. At 640 pixels and below, keep 20
pixels of page margin and let actions stack vertically.

## Elevation & Depth

Most surfaces use tonal layering and one-pixel dividers. Shadows belong to
floating navigation, primary actions, real product windows, and large proof
captures.

### Shadow Vocabulary

- **Navigation lift:** `0 12px 34px rgba(8, 43, 99, 0.12)` keeps the fixed
  header separate from the page.
- **Action lift:** `0 9px 24px rgba(8, 43, 99, 0.24)` gives the primary action
  clear priority.
- **Proof lift:** Soft shadows with a 24-30 pixel vertical offset support
  screenshots and source windows.

**The Flat Reading Rule.** Keep prose and lists flat. Reserve elevation for
objects that represent an interface or an action.

## Shapes

Controls use gently rounded 12-pixel corners. Product panels use 16-pixel
corners. The fixed navigation uses an 18-pixel corner. Small status controls
can use a full pill only when their content is compact.

## Components

### Buttons

- **Shape:** Gently rounded control with a 50-pixel minimum height.
- **Primary:** Deep-blue fill, light text, and compact horizontal padding.
- **Hover / Focus:** Darken the fill, lift by one pixel, and keep a visible
  two-pixel focus outline.
- **Text action:** Use muted blue text with an underline on hover.

### Cards / Containers

- **Corner Style:** Use 16-pixel corners for product windows and screenshots.
- **Background:** Use cool paper, source blue, or terminal navy according to
  the content.
- **Shadow Strategy:** Apply proof lift only to a product or source surface.
- **Border:** Prefer one divider or one shadow. Do not combine both by default.

### Navigation

Use a fixed translucent light-blue topbar with the logo, release status, and
three concise destinations. Preserve visible focus and reduce labels before
removing navigation access on small screens.

## Do's and Don'ts

### Do:

- **Do** show real source files, viewer states, and terminal output.
- **Do** vary page rhythm between large fields, quiet prose, and product proof.
- **Do** keep one source path or command readable when a technical visual is central.
- **Do** use SVG icons from the established stroke system.

### Don't:

- **Don't** turn the page into a grid of interchangeable feature cards.
- **Don't** use monospace as decoration outside code, paths, versions, or data.
- **Don't** invent customer proof, benchmarks, or product behavior.
- **Don't** use gradient text, decorative glass, or a second accent palette.
