---
version: 1
slug: "docs-site-src-pages-index-js"
primary_target: "docs-site/src/pages/index.js"
related_targets: ["docs-site/src/css/custom.css","docs-site/docusaurus.config.js"]
---

Scope: the bazNTMS docs-site (`docs-site/`, Docusaurus 3.4 → GitHub Pages). Primary
surface = the landing page (`docs-site/src/pages/index.js` + `index.module.css`).
Related = the Docusaurus shell reskin (`docs-site/src/css/custom.css`,
`docusaurus.config.js`) and the three hand-written doc pages
(`docs-site/docs/{intro,installation,agent-deployment}.md`).

Visitor mode: **Persuade** for the landing page; **Read** for the doc pages
(shell reskin serves both).

## Established world — inherited, not invented

The product ships a thoroughly documented visual world: `frontend/DESIGN.md` —
"htop çok-panelli terminal". Flat phosphor-black `#0a0d13`, single JetBrains Mono
family everywhere (long article prose is the only opt-out → system sans), square
corners (radius 0), zero shadow, opaque surfaces, no blur, no dot-grid. Seven
saturated ANSI-like accents each pinned to exactly one operational meaning
(rx-cyan `#22d3ee`, tx-violet `#a78bfa`, emerald `#34d399` healthy, amber
`#fbbf24` warn, rose `#fb7185` critical, sky `#38bdf8` proc-alarm, orange
`#fb923c` vendor). Signature components: `Panel` (`┤ TITLE ├` box-title),
`Meter` (`LABEL [███····] value`), `TuiTable` (reverse-video cyan header strip),
`Sparkline` (`▁▂▃▄▅▆▇█`), `StatusPill` (`● / ○`), `TabBar`/`FnKeyBar`
(reverse-video). Static CRT scanline (`body::after`, 1px/3px), blink block cursor
on live values. Keyboard-first: `1–9` switch tabs, `F1–F10` bottom strip.

This surface inherits that world unchanged. No concept roll: the world is settled
and in production across ~40 dashboard pages; the composition below is
user-approved in the plan at `~/.claude/plans/goofy-moseying-parnas.md`.

## Audience & job

- **Landing visitor:** a NetOps / security engineer or IT lead evaluating a
  self-hosted NTMS. Must grasp in seconds: what it is (hub + agent + device
  monitor with signed 5651 logs), why it is different (the triple architecture +
  hash-chained compliance + one binary from 1 node to 5 000 agents), and what to
  do (`[ KURULUM ]` / copy the compose command / GitHub).
- **Doc reader:** operator installing or upgrading. Wants the command, the flag,
  the table — fast scanning, then a reading experience worth staying in.
- **Proof on hand (real):** the actual dashboard (quoted as a static Panel
  composition), the architecture diagram, verified capacity numbers
  (`docs/CAPACITY.md`), the v1.3.0 changelog. No customer logos, no invented
  benchmarks — none exist.

## Must remain untouched

Docusaurus as the engine; `sync-docs.mjs` + `sidebars.js` + the auto-synced
`docs/reference/*` page **content**; `frontend/`; `.github/workflows/docs.yml`.
Turkish copy + comments. `onBrokenLinks: 'warn'` — repo-only docs get absolute
GitHub URLs (MDX relative-link fragility).

## What would make a polished result feel wrong

A dark-slate + cyan-glow dev-tool landing (gradient hero, eyebrow kickers, 12
icon-tile cards, "what's new" pills, glass) — which is exactly what the page is
now and what impeccable's own 2026-09-04 critique flagged (ai-color-palette 87
hits). Any rounded corner, shadow, gradient, or blur. Monospace used as costume
rather than as the product's real single-family system. A colored `border-left`
that is decorative rather than the documented Triad semantic accent.

## Direction contract

THESIS: the landing page is a bazNTMS screen you operate, not a brochure about
one — you read it panel by panel and drive it with `1–9` / F-keys, the same as
the product. Refuses the dark-slate + cyan-glow dev-tool template: no gradient
hero, no eyebrow kicker, no icon-tile card grid, no "what's new" pill wall.

OWN-WORLD: `frontend/DESIGN.md` verbatim — phosphor-black `#0a0d13`, one
JetBrains Mono family, square corners, no shadow/blur, seven fixed-meaning ANSI
accents, `Panel` box-titles, bracketed `Meter` bars, reverse-video `TuiTable` and
Fn-key strip, `▁▂▃▄▅▆▇█` sparklines, static scanline, block cursor. Content
removed, it still reads as a terminal monitor, not a SaaS page.

STORY: visitor lands inside a running `Genel Bakış` screen (live meter band +
throughput trace), pages down through `Yetenekler` (a filterable-looking
`TuiTable`, not cards), the real `Dashboard` quoted as a static Panel, the 5651
signing chain as box-drawing links, the architecture screen, the scale table, a
`man`-page install screen; believes this is a real operator tool run by people
who read terminals; acts via the always-visible `[ KURULUM ]` / `F2`.

FIRST VIEWPORT: full-bleed `#0a0d13`. Thin top strip: `◇ bazNTMS` · `● WS:CANLI`
pill · compact RX/TX/PPS `Meter` band · `[v1.3.0]` · live clock + blink cursor.
Two columns: LEFT ~55% — one mono positioning line ("Paketten imzalı kayda. Tek
makineden 5 000 agent'a, aynı binary."), a `$ docker compose … up --build` copy
row, then `[ KURULUM ]` (reverse-video cyan, primary) + `[ GITHUB ↗ ]`. RIGHT
~45% — `┤ CANLI ÖZET ├` Panel: RX/TX `Meter`s + hand-SVG step-line throughput
chart (synthetic, one rAF, freezes under reduced-motion), caption "temsilî —
gerçek veri değil". Pinned bottom: reverse-video `FnKeyBar`
(`F1 YARDIM  F2 KURULUM  F3 API  F5 GITHUB`). Primary action top-left + in the
pinned strip.

FORM: operated-terminal-screen composition (my ranked structures: 1. paged
monitor screens, 2. man-page column, 3. datasheet spec sheet, 4. changelog
timeline, 5. annotated dashboard poster). No seed key — established world,
user-pinned composition, shaped directly per new-work.md §3.

FINISH: unreviewed and undocumented is unfinished; this build ends with the
finish review, the verdict, DESIGN.md, and every shipping raster carrying its
provenance.

## Unresolved

- FnKeyBar on mobile: horizontal-scroll strip (matches real `FnKeyBar`), keep
  ~24px tall.
- `docs-site/DESIGN.md` to be written at finish by the documenter (docs-site's
  rendition of the TUI world: Infima integration, article prose-sans, landing
  composition) — there is no root/ docs-site DESIGN.md today.
