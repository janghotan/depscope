---
name: DepScope
description: A restrained inspection desk for package evidence and coverage.
colors:
  safety-orange: "#f1855c"
  safety-orange-light: "#ffad8c"
  signal-amber: "#f0c46a"
  inspection-green: "#74b997"
  inspection-green-light: "#8ed0ad"
  shell: "#0d1519"
  shell-raised: "#17252c"
  shell-edge: "#33434a"
  sheet: "#152229"
  paper-text: "#dce5e5"
  paper-deep: "#203039"
  ink: "#e5eceb"
  ink-soft: "#b8c5c8"
  ink-faint: "#91a1a6"
  rule: "#394a51"
  rule-strong: "#586b72"
  night: "#0b1114"
typography:
  headline:
    fontFamily: '"Overpass Variable", "Overpass", ui-sans-serif, system-ui, sans-serif'
    fontSize: "2.15rem"
    fontWeight: 700
    lineHeight: 1.02
    letterSpacing: "-0.025em"
  body:
    fontFamily: '"Overpass Variable", "Overpass", ui-sans-serif, system-ui, sans-serif'
    fontSize: "1rem"
    lineHeight: 1.65
  label:
    fontFamily: '"Overpass Mono Variable", "Overpass Mono", ui-monospace, SFMono-Regular, Menlo, monospace'
    fontSize: "0.6875rem"
    fontWeight: 500
    lineHeight: 1.2
    letterSpacing: "0.16em"
  data:
    fontFamily: '"Overpass Mono Variable", "Overpass Mono", ui-monospace, SFMono-Regular, Menlo, monospace'
rounded:
  square: "0px"
components:
  button-primary:
    backgroundColor: "{colors.shell}"
    textColor: "{colors.paper-text}"
    rounded: "{rounded.square}"
    padding: "0.75rem 1rem"
    height: "3rem"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.square}"
    padding: "0.5rem 0.75rem"
    height: "2.5rem"
  field:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.square}"
    padding: "0.7rem 0.8rem"
    height: "3rem"
  navigation-active:
    backgroundColor: "transparent"
    textColor: "{colors.paper-text}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    height: "2.5rem"
  status-tag:
    backgroundColor: "transparent"
    textColor: "{colors.inspection-green-light}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0.15rem 0.45rem"
    height: "1.5rem"
  record-sheet:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.square}"
    padding: "1rem"
  condition-stamp:
    backgroundColor: "transparent"
    textColor: "{colors.safety-orange-light}"
    typography: "{typography.data}"
    rounded: "{rounded.square}"
    padding: "0.55rem 1rem 0.5rem"
---

# Design System: DepScope

## Overview

**Creative North Star: "Inspection Bay"**

DepScope is a working inspection desk for package condition records. A quiet blue-black shell and restrained instrument details keep attention on the report: its condition, confirmed evidence, heuristic signals, successful checks, and coverage gaps. The interface treats the condition stamp as an index into the evidence, never as a substitute for it.

The visual language is technical and legible: Overpass for interface text, Overpass Mono for labels and package data, square outlined sheets, fine rules, and sparse safety-orange markers. The desktop and mobile views retain the same evidence order; narrow layouts stack their panels and keep comparison overflow inside the table.

**Key Characteristics:**
- Dark, low-glare inspection workspace.
- Strong separation between findings, heuristics, and missing coverage.
- Square, ruled surfaces with restrained orange, amber, and green signals.

## Colors

The palette is blue-black and slate, with pale text and three limited inspection signals.

### Primary
- **Safety Orange**: The inspection action, visible focus, and confirmed-risk emphasis use the warm orange accent.
- **Safety Orange Light**: Used for stronger text contrast on dark surfaces and confirmed evidence labels.

### Secondary
- **Signal Amber**: Marks caution, incomplete coverage, and demo data; it also anchors the specimen-sheet header.

### Tertiary
- **Inspection Green**: Identifies successful checks and completed inspection signals.
- **Inspection Green Light**: Provides the brighter success color for small labels and icons.

### Neutral
- **Blue-Black Shell** and **Raised Shell**: Frame the page and header; raised slate separates interactive areas.
- **Sheet Slate**: The base for record panels and fields.
- **Paper Text** and **Deep Paper**: Pale text and a deeper slate for secondary blocks and code.
- **Ink**, **Soft Ink**, and **Faint Ink**: Primary, supporting, and hint text on dark surfaces.
- **Slate Rule** and **Strong Rule**: Separate records and define controls without heavy boxes.
- **Night**: Dark text on amber or orange signal fills.

**The Evidence Color Rule.** Color reinforces a written status or evidence label; it never carries severity, status, finding kind, or demo mode by itself.

## Typography

**Display Font:** Overpass Variable (with Overpass and system sans-serif fallbacks)

**Body Font:** Overpass Variable (with Overpass and system sans-serif fallbacks)

**Label/Mono Font:** Overpass Mono Variable (with Overpass Mono and system monospace fallbacks)

**Character:** Overpass keeps explanatory copy direct and readable. Overpass Mono makes identifiers, timestamps, counts, commands, and instrument labels read as record data.

### Hierarchy
- **Headline** (700, 2.15rem / 3rem from 640px, 1.02 line-height): Page-level inspection statement; kept compact and tightly set.
- **Title** (700, 1.25rem, tight tracking): Section titles such as Findings and comparison headings.
- **Body** (400, 1rem, up to 1.65 line-height): Explanations and report prose; secondary copy uses softer ink.
- **Label** (500, 0.6875rem, 0.16em tracking, uppercase): Instrument labels, navigation, and section metadata.
- **Data** (Overpass Mono Variable, tabular numerals): Package names, versions, timestamps, counts, and commands.

**The Data Voice Rule.** Keep package identifiers and measured values in monospace; keep explanations in the interface sans serif.

## Layout

The shared record container is fluid up to 90rem, with horizontal gutters of 1rem by default, 1.5rem from 40rem, and 2.5rem from 64rem. The home inspection request and demo specimen sit in a two-column layout at the large breakpoint; report evidence also pairs its main ledger and load path with checks, sources, and limitations. At smaller widths these regions stack in reading order. The site header also changes from stacked rows on mobile to a single instrument bar on wider screens.

Use compact, explicit groupings and preserve the record sequence: condition, findings, dependency path, checks, sources, limitations. Comparison tables scroll within their own container on narrow screens. Fine survey ticks and horizontal rules provide measurement cues without adding dashboard chrome.

## Elevation & Depth

The interface is flat at rest and has no box-shadow vocabulary. Depth comes from blue-black versus slate surfaces, outlined sheets, top-edge markers, and fine rules. State changes use color or border contrast rather than glow; the orange sweep is reserved for real scan progress.

**The Flat Instrument Rule.** Keep sheets flat and square; use tonal layering and rules to distinguish structure.

## Shapes

Controls, tags, sheets, and status stamps use square corners (0px radius). Borders are thin and structural; the specimen sheet and stamped condition use heavier outlines where they need instrument-like emphasis. Load-path nodes use a vertical rule with short branches, while small square markers indicate checkpoints. Avoid pill shapes and rounded card grids.

## Components

### Buttons
- **Character:** Clear, square actions with a measured orange edge.
- **Primary:** Dark shell fill, pale text, 1px shell border, 3px orange bottom edge; 0.75rem 1rem padding and 3rem minimum height.
- **Hover / Focus:** Hover fills with safety orange and uses night text; keyboard focus has a 2px orange outline with 2px offset.
- **Secondary:** Transparent fill, strong slate border, 0.5rem 0.75rem padding, and 2.5rem minimum height; hover adds a raised-shell fill.
- **Disabled:** Muted text and stronger border distinguish unavailable actions.

### Chips
- **Style:** Uppercase monospace labels with a 1px outline and compact padding; severity fills use readable foreground colors.
- **State:** Confirmed and heuristic labels remain text-labelled, with heuristic outlines dashed. Unknown severity uses a dashed neutral outline.

### Cards / Containers
- **Corner Style:** Square (0px radius).
- **Background:** Sheet slate over the blue-black shell; secondary blocks use deep paper slate.
- **Shadow Strategy:** None; use borders, rules, and tonal contrast.
- **Border:** Strong slate outline for sheets, thin rules within records, and an optional dark top edge on primary sheets.
- **Internal Padding:** Common panels use 1rem, increasing to 1.5rem at the small breakpoint; report sections use generous vertical separation.

### Inputs / Fields
- **Style:** Sheet fill, strong slate stroke, square corners, monospace input text, 0.7rem 0.8rem padding, and 3rem minimum height.
- **Focus:** Orange 2px outline with 1px offset and a brighter border.
- **Error / Disabled:** Invalid fields use oxide-orange borders; errors are written out below the field. Placeholder text uses faint ink.

### Navigation
- **Style:** Compact uppercase monospace links. The active page receives a 2px orange bottom rule and pale text; inactive links use softened text and gain a slate rule on hover.
- **Mobile:** Navigation occupies a second row below the brand and status indicator, with evenly sized touch targets.

### Condition Stamp
- **Style:** A square, two-pixel outlined monospace stamp, slightly rotated (-1.25deg), with an icon and written condition. Status color varies, but the condition remains explicit text.
- **Motion:** The optional entrance press settles into place; reduced-motion preferences shorten animation and transition durations.

### Evidence Path
- **Style:** Nested ordered nodes follow a thin vertical branch rule. Orange marks findings, amber marks heuristic signals, and selecting a node reveals its package details and associated evidence.
- **Behavior:** Opening a path finding also opens and focuses its evidence record so the relationship remains traceable by keyboard.

## Do's and Don'ts

### Do:
- **Do** keep the condition, findings, path, checks, sources, and limitations visibly distinct.
- **Do** keep status and finding labels readable without color and preserve visible keyboard focus.
- **Do** use orange sparingly for inspection actions, focus, and confirmed-risk emphasis; reserve amber for caution and demo labeling.
- **Do** honor reduced-motion preferences and keep narrow-screen tables in local scroll containers.

### Don't:
- **Don't** imply that missing or failed checks indicate a clean condition.
- **Don't** blur confirmed advisories and heuristic signals into one category.
- **Don't** add rounded dashboard cards, gradients, or glow effects to the flat inspection surfaces.
- **Don't** present demo fixtures as live scan results.
