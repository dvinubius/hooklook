# The hooklook mark

The product mark is the webhook glyph plus a magnifying glass, set beside the
wordmark. It replaces the earlier `[ hooklook ]` bracket idiom in the UI; the
personal wordmark `[ Dinu Barbu ]` in the footer keeps its brackets.

Two marks came out of the exploration. **B·2 is the one the UI ships**
(`frontend/src/components/BrandMark.vue`). **C·1 is kept here** as the
alternate: it reads better as a standalone badge, where no wordmark carries the
name.

Both are drawn on the 24-unit grid of the webhook icon they start from, with
round caps and joins throughout, the glyph in `--text-body` and the lens in
`--accent` — Ember `#A8500F` on light, Ember Light `#DE8A42` on dark, per the
accent pair rule in the Dinu Barbu design system.

## The source glyph

The three-armed webhook icon, unchanged, stroke 2:

```
M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2
M6 17l3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06
M12 6l3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8
```

Its three nodes are hooks — open arcs of radius 4 centred on `(6,17)`,
`(12,6)` and `(18,17)` — and each arm leaves from the centre of the node
before it.

## B·2 — the shipping mark

The whole glyph stays drawn. A lens of radius 6.5 lies over the bottom-right
hook; the hook shows through the glass at 60% opacity, and the arms pass under
the ring with a 0.9-unit gap of page colour on each side, so the lens reads as
lying on top rather than cut into the glyph.

- ring r 6.5 centred `(18,17)`, stroke 2
- handle stroke 2.6, from the ring outwards at 45°
- glass fill `--accent-glyph` (the accent at 13% on dark, 8% on light)
- viewBox `1 0.5 25.44 25.44` — square and centred on the glyph, so aligning
  the box aligns the mark

## C·1 — the alternate

The whole glyph sits inside one large lens, concentric with it.

- ring r 8.2 centred `(12,12)`, stroke 2
- handle stroke 2.6, from the ring outwards at 45°
- glyph scaled 0.5589 about `(12,13.133)`, stroke 3.042 in its own units
  (1.7 as drawn)
- viewBox `1.22 1.22 21.55 21.55`

```svg
<svg viewBox="1.22 1.22 21.55 21.55" xmlns="http://www.w3.org/2000/svg">
  <g fill="none" stroke-linecap="round" stroke-linejoin="round">
    <circle cx="12" cy="12" r="8.2" stroke="var(--accent)" stroke-width="2" />
    <g transform="translate(12 12) scale(0.5589) translate(-12 -13.133)"
       stroke="var(--text-body)" stroke-width="3.042">
      <path d="M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2" />
      <path d="M6 17l3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06" />
      <path d="M12 6l3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8" />
    </g>
    <path d="M18.081 18.081L21.475 21.475" stroke="var(--accent)" stroke-width="2.6" />
  </g>
</svg>
```

### Why the glyph is scaled the way it is

The glyph is a triangle, so its bounding-box centre is not its optical centre.
Fitting it by bounding box left it visibly off inside the ring. The scale above
comes from the smallest circle that encloses the glyph's strokes — found by
sampling 200 points along each path and searching for the centre that minimises
the enclosing radius. That centre is `(12, 13.133)`, and the radius sets the
scale. Re-derive it the same way if the glyph or the ring size ever changes.

## The rules both marks follow

**The handle never pokes into the glass.** The handle is thicker than the ring
(2.6 against 2), so starting it on the ring's centreline pushes its round cap
about 0.3 units past the ring's inner edge. Each handle starts at
`r + (handle − ring) / 2 + 0.1` instead, which hides the cap inside the ring.

**The mark is centred on the word's ink, not on the x-height.** In the lockup
the mark's box is baseline-aligned and shifted down by
`mark-height / 2 − 0.343em`, which puts the mark's centre 0.343em above the
baseline. That number is measured, not guessed: in Space Grotesk 500 the ink of
"hooklook" runs from 0.700em above the baseline (the tops of `h`, `k`, `l`) to
0.014em below it (the `o`'s overshoot), so its centre is `(0.700 − 0.014) / 2`.
The x-height midline — 0.253em — is 0.09em lower and makes the wordmark look
like it is leaning up out of the mark, because the word has four ascenders and
no descenders.

**Sizes in the lockup.** B·2 takes a 1.15em box, C·1 a 1.3em box (C·1 draws
less ink inside the same square, so it needs the extra), and the gap between
mark and wordmark is 0.38em. At 19px — the header size — B·2's lens still
reads; C·1's inner hook starts to close up below about 18px.

## The favicon

`frontend/public/favicon.svg` — the mark whole, the same drawing the lockup
uses, on an Ink tile.

- 32-unit tile, Ink, corner radius 7 (22%), 3 units of margin
- the glyph scaled 1.022 about the tile, Paper at stroke 2
- ring r 6.5 stroke 2 in Ember Light, glass `#DE8A4224`, handle stroke 2.6
- the hook inside the glass in Paper at 60%, clipped to r 5.2 — the same
  dimming the lockup uses, which over the Ink tile and the glass tint reads as
  a grey rather than white

It stays dark in both themes, and its colours are literal hex rather than
tokens: the file is fetched on its own and never sees the app's CSS variables.

It is linked from both shells — `frontend/index.html` for the build and
`devShell` in `frontend.go` for development — and served by a route of its own,
`GET /favicon.svg`, because it sits at the build root rather than under
`/assets`. Its name is stable rather than content-hashed, so it takes
`staticFileCache`, the same ordinary lifetime the fonts get.

Two crops were tried live in a tab and rejected: the mark zoomed onto its lens
with the arms running off the tile, and the lens alone with everything outside
the ring dropped and the ring and handle centred in the tile. Both are more
legible at a true 16px, and both stop being a webhook — they read as a plain
magnifier. Keeping the whole mark keeps the subject, and desktop is the only
target, so 24px and up is where it has to work. Four other candidates were
drawn and dropped: the mark bled to the edges, the C·1 badge, the glyph with no
lens and its bottom-right hook in Ember, and the glyph reduced to three dots
inside the lens.
