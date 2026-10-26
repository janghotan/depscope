# Assets

Public brand and documentation assets for DepScope. All files here are hand-authored for this project, contain no third-party artwork, and use the color tokens defined in `web/src/app/globals.css` (`#0d1519` shell, `#74b997` green, `#f1855c` oxide, `#f0c46a` amber, `#e5eceb` ink). There is no external image dependency; the SVGs are self-contained.

Simple English | 简体中文: every diagram exists in an English and a Simplified Chinese variant with the same layout and the same meaning.

## Layout

| Path | Purpose | Format |
| --- | --- | --- |
| `brand/depscope-mark.svg` | Logo mark: a package square with a dependency trail inside an inspection reticle. Shows four variants (dark background, on green, on light, transparent) for placement guidance. | SVG 560 × 144 |
| `brand/depscope-wordmark.svg` | Mark plus `DepScope` wordmark and the tagline. Includes an opaque dark background so it reads correctly on any GitHub theme. | SVG 520 × 136 |
| `readme/depscope-overview-en.svg` | Architecture: terminal CLI/TUI, HTTP API, and website over one application service, the shared Go engine, the approved sources, and the canonical report. | SVG 1200 × 800 |
| `readme/depscope-overview-zh-CN.svg` | Simplified Chinese version of the architecture diagram. | SVG 1200 × 800 |
| `readme/depscope-scan-flow-en.svg` | The real scan flow: package identity, metadata and advisory checks, evidence-based risk, report. | SVG 1200 × 800 |
| `readme/depscope-scan-flow-zh-CN.svg` | Simplified Chinese version of the scan flow. | SVG 1200 × 800 |
| `social/depscope-social-preview.svg` | Editable source of the GitHub social preview. | SVG 1280 × 640 |
| `social/depscope-social-preview.png` | Validated PNG export for GitHub's social preview setting (recommended 1280 × 640). | PNG 1280 × 640 |

The terminal screenshot referenced by the READMEs is `../docs/assets/terminal-demo.png`; it is a capture of `depscope demo vulnerable` and is labelled as synthetic demo data.

## Visual rules

- Dark neutral surface, restrained green accent, oxide for confirmed-risk emphasis, amber for caution and demo labelling.
- Text carries meaning; color never encodes status or severity on its own. Every label in the diagrams is plain text.
- Monospace is used for package identifiers, commands, and report field names; the interface sans serif is used for headings and explanations.
- No gradients, glow effects, stock-photo imagery, fabricated metrics, or claims about hosting that does not exist.

## Typography and portability

The SVGs reference font families only; no fonts are embedded and no external font files are loaded. Latin text uses `Inter`/`Helvetica Neue`/`Arial`/generic system fallbacks and monospace text uses `JetBrains Mono`/`DejaVu Sans Mono`/`SFMono-Regular`/`Menlo`/`Consolas`. The Simplified Chinese variants list `Noto Sans CJK SC`, `Noto Sans SC`, `Source Han Sans SC`, `PingFang SC`, `Microsoft YaHei`, and `Noto Sans` before the Latin fallbacks, so CJK glyphs resolve on Linux, macOS, and Windows. On a system with no CJK font installed, the Chinese text will fall back to whatever the renderer substitutes; install a CJK font to preview it.

## Regenerating and validating

The assets are checked in as final files. To verify them after editing:

```sh
# XML well-formedness
python3 -c "import xml.dom.minidom,sys; xml.dom.minidom.parse(sys.argv[1])" assets/readme/depscope-overview-en.svg

# Raster preview (librsvg)
rsvg-convert -w 1200 assets/readme/depscope-overview-en.svg -o /tmp/overview.png
rsvg-convert -w 1280 assets/social/depscope-social-preview.svg -o /tmp/social.png
```

GitHub renders README SVGs through its image proxy, so the files must stay self-contained: no `<script>`, no event handlers, no `<image>` references, no external `url(...)` or `@font-face` rules, and a `<title>` for accessibility.

## Uploading the social preview

The PNG is prepared but **has not been uploaded**. GitHub does not accept a social preview through the repository contents; a maintainer uploads it in the repository settings (Settings → General → Social preview → Edit → Upload an image). Keeping the uploaded image identical to this file keeps the published preview consistent with the checked-in asset.
