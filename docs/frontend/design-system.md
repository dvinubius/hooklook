# Visual style

The inspector UI is built in the **Dinu Barbu** brand system. This file is the
authority for how that system is used here.

**It used to say the design system wins any disagreement. It does not any
more.** That system was conceived for a landing and portfolio page — one
column of prose, one accent, no state. A data-rich UI has regions to separate,
interactive states to tell apart, and several kinds of text on one screen, and
the ten adaptations below are what that turned out to require. Reading the
upstream repository as binding would undo every one of them, so the rule is
inverted: for this product's UI, **this file wins**, and the departures are
written down here rather than smuggled into components.

**Where the system still comes from:**
`the Dinu Barbu design-system repository` — `readme.md` is the written spec,
`tokens/` the values, `components/core/` reference implementations,
`guidelines/cards/` the specimen cards. It is also available as a Claude skill
named `dinu-barbu-design`. Treat it as the origin of the brand rather than as
a rulebook: the palette, the two typefaces and the shape language are the
brand and should stay recognisably the same everywhere. How a product UI
spends them is this file's business.

**What is vendored here:** `frontend/src/styles/brand.css` (the color and type
tokens, copied apart from the theme-default adaptation noted below) and
`frontend/public/fonts/*.woff2` (four subset files built by the design
system's `build-webfonts.py`). The raw brand *values* are still worth keeping
in step with upstream — Ink, Paper, Ember and the typefaces are the brand
itself, and drifting them would make two different brands. Values this app
needs and the brand does not have are declared in `app.css` instead, where
they are visibly this app's own: `--text-dim`, `--surface-float`,
`--float-hairline`, `--shade-hairline`, the data colors and the code ramp.

## The binding rules

**Color.** True-neutral base plus one warm accent at two lightnesses. Ink
`#141414` / Paper `#FAFAFA`. Exactly one muted grey per theme: Stone `#6E6E6E`
on light, Muted on Dark `#9A9A9A` on dark — there is no fainter second tier.
This app adds one tier *between* body and muted rather than below it; see
adaptation 6.

**Accent pair rule:** Ember `#A8500F` on light, Ember Light `#DE8A42` on dark.
Never the reverse.

**Expanded palette.** To serve a data-rich app, the palette is stretched past
the brand's single accent. Every hue comes as a light-theme / dark-theme pair,
like the accent:

| | Light | Dark | Role |
| --- | --- | --- | --- |
| Ember | `#A8500F` | `#DE8A42` | brand, important action |
| Teal | `#087581` | `#5BC8D0` | primary data and syntax color |
| Violet | `#6B5D91` | `#AFA3CF` | secondary data, type or category color |
| Brick | `#A03028` | `#E0756A` | errors and danger |

Stone, Ink and Paper still carry the overwhelming majority of the interface.
Teal and Violet are spent on syntax alone, on code surfaces (see below) — a
request path in the list and in the detail's title used to take Teal too, and
reads better as the row's own body text. Brick (`--danger` in `app.css`) only
in the clear-bin confirmation: the solid `.btn-danger` and the dialog's
failure message.

**Accent dosage** is binding: at most ~2% of any composition, never on running
text, never the sole carrier of a UI state, roughly one accent moment per view.
In this UI the accent is spent on the lens of the mark, the single
primary action, the selected-row rule and the copy control inside a link
field — and a state that uses it always carries a text label too.

**Type.** Space Grotesk (headings, body, the mark; 400/500/700) and Azeret Mono
(everything technical: codes, paths, headers, bodies, `//` asides, meta labels;
400/500 only). Mono renders at 0.93× its nominal size. Space Grotesk has no
true italic — emphasis is weight or accent, never synthesized slant. Headings
are 500 at −0.022em. Sentence case everywhere; the brand sets mono meta-labels
lowercase, and this app sets them in caps — see adaptation 10.

**Shape.** The brand has square corners everywhere; this app softens them a
little (see the adaptations below). No shadows, inner or outer. No gradients,
textures or background imagery. Separation is 1px hairlines (`#E6E6E6` light /
`#2C2C2C` dark, non-text only) and flat neutral fills. Cards are a neutral fill
with no border and no shadow. Floating layers are the one case where no-shadow
needs help; see adaptation 7.

**Code and terminal surfaces** follow the theme — a departure from the brand,
which keeps them dark in both. On dark pages code sits on Panel `#1C1C1C`
with a hairline; on light pages on Shade `#F0F0F0` with no border. Each theme
has its own brightness ramp (the `--code-*` tokens in `app.css`), and the data
colors take the theme's variant. Captured bodies are highlighted with the data
colors; other code surfaces (the example request, links) keep the ramp:

| Tier | Light | Dark | Used here for |
| --- | --- | --- | --- |
| name | Violet `#6B5D91` | Violet `#AFA3CF` | JSON keys, XML element and attribute names |
| value | Teal `#087581` | Teal `#5BC8D0` | JSON strings, numbers, literals; XML attribute values and text |
| emphasis | Ink `#141414` | Paper `#FAFAFA` | emphasis outside highlighted bodies |
| body | `#3D3D3D` | Muted Strong `#B5B5B5` | text, output |
| recede | Stone `#6E6E6E` | Muted on Dark `#9A9A9A` | punctuation, tags, comments |
| accent | Ember `#A8500F` | Ember Light `#DE8A42` | at most one line, often none |

The light body tier `#3D3D3D` is the one neutral here that is not a brand
value: the brand's light theme has no grey between Ink and Stone. It is the
same value as the page's dim tier (adaptation 6) and is declared once, as
`--text-dim`.

Highlighting is produced as token data and rendered through text
interpolation — captured bytes never reach the DOM as markup.

**Request headers are a table, not a code surface:** one header per row on
the page itself, parted by hairlines like the request list, mono throughout.
Names are body text, values the dim tier.

**Motion.** None is defined in the brand. Default to no animation; if something
must move it is opacity-based and imperceptibly fast. Never bounces. A
reconnecting stream says so in words rather than pulsing. There are two
exceptions. A pending owner action — saving the access setting — shows a small
turning arc (`LoadingSpinner`) beside its control. Hover changes on buttons
and links — color, background, border — ease in and out over 150ms
(`--hover-transition` in `app.css`); a switch's knob slides with the same ease.
List items — request rows, header rows, menu options — change instantly.
Everything else changes instantly.

**Buttons and links.** Primary: solid accent fill, square. Secondary: 1px
outline. Quiet link: text with a 1px accent bottom border and a trailing `→`.
Never underline a button. A disabled control keeps its shape and drops to
`--disabled-opacity` with `cursor: not-allowed` — one treatment, spent by
`.btn:disabled`, the access switch and the row delete control alike.

**Glyphs.** The brand has no icon system and no emoji, deliberately: Unicode
does icon duty — `↳` `·` `→` `×` `✓` `//` `[ ]`. `×` and `✓` are a valence pair
used together in figures and marked lists, never in running prose or as a lone
decorative tick. All of them really ship in the vendored font subsets. This app
adds a small set of line icons for its controls (see the adaptations below).

## Adaptations for this app

The brand has no product UI precedent — "the product is the brand itself" — so
these decisions were made here and should stay consistent:

1. **Dark is the default.** The design system ships light as the bare `:root`
   default with dark under `[data-theme="dark"]`. `brand.css` instead declares
   the semantic aliases under both explicit theme attributes, and the page
   shell always carries one. Raw palette values are unchanged.
2. **The product mark** is the webhook glyph under a magnifying glass, set
   beside the name in the wordmark idiom — the lens is the accent, the glyph
   takes the body colour, and the brackets the name used to wear are gone.
   `BrandMark.vue` draws it; [brand mark](brand-mark.md) holds the geometry
   and the alternate mark. The personal wordmark `[ Dinu Barbu ]` keeps its
   brackets and belongs in the footer, not the header; as in zibs, the footer
   pairs it with a quiet `→ dinubarbu.com` link. Between them, centred, the
   muted `↳ dvinubius` credit links to the source on GitHub.
3. **A small icon set.** Icon-only controls that a label would crowd — help,
   sweep, share, copy, delete, GitHub — are drawn as `Icon*.vue` components:
   square-cut 24px line drawings, `currentColor`, stroke 1.5, no fill. They
   take the size of the row they sit in (20px in the capture row, 16px in a
   list row). Everything else still uses the Unicode glyphs above.
4. **An expanded palette and theme-aware code surfaces**, both described
   above — the app is data-rich, and the brand's single accent could not
   carry syntax or status on its own.
5. **Softened corners.** Controls — buttons, fields, dropdown triggers, the
   segmented control, the switch — take `--radius-control` (2px); surfaces —
   code blocks, dialogs, popovers, dropdown menus, filled blocks —
   `--radius-surface` (2px). Rows in a list stay square: they are parted by
   hairlines, not boxed. Both tokens live in `app.css`.
6. **Three text tiers on the page, not two.** With only body and muted,
   everything that was not primary fell the whole way to muted — running
   prose included, which on light is 4.89:1 and below AA at 13px. The middle
   tier is not a new value: it is the one the code ramp already carried for
   its body text, lifted out as `--text-dim` so the page can spend it too.
   The brand's rule is untouched — there is still no tier *fainter* than the
   one muted grey.

   | Tier | Light | Dark | Used for |
   | --- | --- | --- | --- |
   | `--text-body` | Ink, 17.6:1 | Paper, 17.6:1 | headings, section labels, the request line, header names, a chosen option |
   | `--text-dim` | `#3D3D3D`, 10.4:1 | `#B5B5B5`, 9.0:1 | prose, notes, facts, control labels, header values, menu options |
   | `--text-muted` | Stone, 4.9:1 | Muted on Dark, 6.6:1 | `//` asides, timestamps, query strings, placeholders, resting icons |

   Two rules go with it. **A section label outranks what sits beside it:**
   `.caps` is body text, and the facts on its row — the stream state, the
   counts, a body's size and format — take `.fact`, which is dim. Before this they were
   the reverse, and the label naming a region was the faintest text in it.
   **Emphasis inside prose is weight, and on dark a lift as well:** `.key`
   in the help guide is `font-weight: 500`, the brand's medium. On light
   that is enough on its own — dark ink on a light page, where a heavier
   stroke is plainly more ink. Light text on a dark page blooms, so the same
   step barely tells, and dark adds `--text-body` as a second channel, 1.96×
   the contrast of the dim tier the prose sits in. Violet was tried there
   first and does not work: it is 0.88× the prose on dark and 0.54× on
   light, a hue shift with no brightness behind it, and it collides with the
   redaction marker. The lift is a step *above* the reading tier, not the
   substitute for it that it was when every note was muted. An unselected
   control state — a segmented control's inactive half, an icon at rest —
   is still muted: that is a state pair, not a tier.

7. **Floating layers get a surface step and a Stone edge.** Popovers, the
   dropdown menu, the cut-name hint and the modal were all the page's own
   colour with a hairline around them. That is where the brand's no-shadow
   rule really costs something: a panel the exact colour of what it covers
   has only a hairline to say it is in front. `--surface-float` gives them a
   fill of their own and the border goes to Stone.

   | | Light | Dark |
   | --- | --- | --- |
   | `--surface-float` | Paper — unchanged | `--surface-card` `#262626`, 1.22:1 off the page |
   | border | Stone, 4.89:1 | Muted on Dark, 5.38:1 against the fill |

   **Only dark gets the fill.** Light has no room: Paper down to the hairline
   is the whole range, and a float at `#EDEDED` would land *under* the
   `#F0F0F0` code surfaces that sit inside these layers — a raised panel with
   its code blocks floating above it — while dropping muted text to 4.36:1,
   below AA. On light the border carries it alone.

   Two consequences, both load-bearing:

   - `--float-hairline` is the hairline *inside* a float on dark. A `#2C2C2C`
     rule on a `#262626` fill is 1.08:1 and gone, so a dialog's outline
     buttons would lose their edge; `#3A3A3A` restores the 1.33:1 a hairline
     has against the page. `ModalDialog` redefines `--hairline` for its whole
     subtree, so everything inside picks it up.
   - **A fill inside a float steps off the float, not off the page.** The
     dropdown's active option and the dialog's failure block used
     `--surface-shade`, which is *below* the float on dark and so read as
     holes. They mix from `--surface-float` toward `--text-body` instead,
     which moves away from the surface in either theme. Code surfaces are the
     exception and want no change: inset is what they are for, and against
     the dark float they finally read as blocks (1.13:1, up from 1.08:1 on
     the page).

   The modal takes the fill but keeps a hairline rather than a Stone edge —
   it is the only float with a backdrop, and a 600px panel outlined in Stone
   on top of a scrim would be the loudest thing on the page.

8. **The shade ramp has two rungs, and the request list is a filled region.**
   `--surface-shade` was one rung doing every filled thing on the page — row
   hover, selection, placeholders, error blocks, the failure banner — and on
   light it is the same value as the code surface. One rung cannot separate a
   region from what sits inside it, which is why the list and the detail
   stayed on the same ground with only a hairline down the gutter between
   them.

   The rule: **a fill steps off its own ground, not off the page.**

   | Rung | Used for |
   | --- | --- |
   | `--surface-shade` | a filled region — the request list — and any block whose ground *is* the page: the detail's placeholders, the failure banner, a header row's hover |
   | `--surface-shade-2` | a fill inside a filled region: the list's row hover, selection and state blocks |

   Rung 2 is 1.10:1 off rung 1 on dark and 1.08:1 on light — the same step
   rung 1 has off the page, so a row in the filled list feels the way a row
   on the page did. It is derived from rung 1 with `color-mix` on `:root`,
   which is also where `data-theme` lives, so it re-resolves on a theme
   change.

   **The list is filled, not the detail**, and the numbers force it. The
   detail holds the code surfaces, and a filled detail collides with them in
   both themes: on dark the pane would land on `#1C1C1C`, which *is* the code
   surface (1.000:1, the blocks vanish); on light the code surfaces would have
   to drop to `#EAEAEA`, 1.04:1 from the hairline colour. Filling the list
   leaves the detail and its code on untouched ground, and it is the smaller
   column, so less of the workspace carries a tint. The hairline that used to
   run down the gutter is gone — the fill edge is the separation now.

   `--shade-hairline` exists for the same reason `--float-hairline` does: the
   page's hairline is tuned to the page, and on rung 1 it falls from 1.196:1
   to 1.095:1 on light, which the list cannot afford when hairlines are what
   part its rows. `RequestList` redefines `--hairline` for its subtree, so the
   row rules, the filter fields and the outline buttons inside it all adapt.

9. **Display sizes above the brand's tokens.** The brand's tokens — 30 / 19 /
   15 / 13 sans, 13 / 12 / 11 mono — cover the running text, and this app uses
   them for it: `--text-small` for prose and buttons, `--text-mono-meta` for
   facts, rows and control labels, `--text-mono-micro` for timestamps and the
   footer credit. They do not name the larger display roles a product UI
   needs, so a few explicit sizes sit above them:

   | Size | Role |
   | --- | --- |
   | 24px sans | the product mark |
   | 20px | the page title ("Your bin"), and a dialog's title |
   | 18px | the detail's request line — method and path |
   | 16px | the section labels (REQUESTS, HEADERS, BODY), and the footer wordmark |
   | 14px | the request list's counts line, and the help dialog's section labels |

   This is a record of what this app settled on by eye, not a scale anything
   else has to adopt. A token is still the first thing to reach for where one
   fits, because that is what keeps the running text consistent — but a
   display role is allowed its own number, and a new one does not need
   permission from this table. The brand's scale was drawn for a page of
   prose; a screen with a list, a detail, dialogs and popovers on it at once
   needs more steps than that, and taking them here is not drift.

   `.caps` carries no size at all: it sets the treatment and each use sets
   the size, because the same treatment reads at 14px in the help dialog and
   20px on a dialog title.

10. **Section labels are set in caps.** The brand sets mono meta-labels
    lowercase. In a product UI they are doing a different job — naming a
    region of a working screen rather than annotating a page — and caps with
    0.12em tracking is what separates a label from the facts beside it
    without spending brightness or a colour on the difference. `.caps` is the
    class; the text is written lowercase in the template and the uppercasing
    is CSS, so a screen reader is not handed shouting.

11. **Shared roles live in `app.css`, and a role carries its tier.** Text
    doing the same job in more than one place is named once rather than
    re-declared per component:

    | Role | Set as | Used for |
    | --- | --- | --- |
    | `.comment` | mono, `--text-mono-meta`, muted, 0.01em | the machine's own `//` asides — nothing captured yet, no body, bin expired |
    | `.caps` | mono, 500, 0.12em, uppercase, body | section labels, dialog titles, guide headings; carries no size (adaptation 9) |
    | `.fact` | mono, `--text-mono-meta`, dim | a count, a figure, a control label, beside the label it belongs to |
    | `.note` | inherited sans, `--text-small`, `--leading-small`, dim, margin-free | prose meant to be read: dialog notes, the help guide, a page explaining itself |
    | `.micro` | mono, `--text-mono-micro` | timestamps, the truncation note — size only |

    The rule: **a role carries a tier when its meaning implies one, and
    carries size alone when its callers legitimately differ.** `.comment` is
    incidental by definition and `.note` is there to be read, so each owns its
    tier. `.micro` does not: its two uses are a dim truncation note and a
    muted timestamp, so each says which with `.dim` / `.muted`.

    What the rule is for: before it, five components had re-declared the note
    treatment under three different names, and six places had re-derived
    `.fact` — four by applying `.micro` and overriding both its size and its
    colour straight back. A role whose tier is wrong for half its callers gets
    overridden rather than reused, and the overrides are invisible until
    someone counts them.

    Two non-text roles keep the same discipline: `.page-state` centres a
    whole-page state — an expired bin, the service at capacity — in the room
    the shell's bars leave, and `--disabled-opacity` is the single disabled
    treatment described above.

    `.comment` and `.caps` were called `.meta` and `.meta-caps`. Both names
    read as size buckets rather than as the one job each does, which is what
    let a second 12px mono role drift in beside them unnoticed.

**HTTP methods are not color-coded.** The brand has one accent and no status
palette, and the compression-constraint habit is to lead with a brightness gap
rather than a hue gap. Methods are distinguished by weight and brightness.

**One status hue.** The single exception to the palette is a live request
stream, shown as "streaming" and a still, round presence-green dot
(`#2BAC76`) at the right end of the request list's header row, the way chat
apps show someone online. The dot stays when the stream is not live: same
shape and size, filled with the muted grey instead, beside "connecting…". So
the row keeps its width whatever the state, and the hue is still the only one
spent on status — grey is the neutral the rest of the page already uses. The
word carries the meaning either way, and every other stream state stays in
words alone.
