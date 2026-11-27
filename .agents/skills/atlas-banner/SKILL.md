---
name: atlas-banner
description: Update or regenerate the Atlas GitHub README banner using the GoForj design system. Use for changes to the banner artwork, wording, or PNG export.
---

# Atlas README banner

Edit `docs/assets/banner.html` and regenerate `docs/assets/banner.png`. The
HTML composition uses the GoForj docs checkout's
`docs/public/design-system.html` as the design reference, with its live CSS
tokens, typefaces, and seam mark.

Keep the map organized around distinct capabilities. Treat `make:*` as a
command family and label individual commands as examples. Verify featured
commands, tool names, and wiring against the GoForj CLI and Atlas source.
Keep the distinction between Atlas guidance and source changes made by
the CLI. Update the README image alt text when the message changes.

From the Atlas root, run the following with Node.js, Chrome, and the GoForj
docs checkout's Playwright dependencies installed. The script loads the
design-system CSS and mark from that checkout and fetches the design fonts.

```sh
node .agents/skills/atlas-banner/scripts/capture.mjs
```

Set `GOFORJ_DOCS_DIR` if the docs checkout is outside `~/code/goforj-docs`.
Pass an output path as the first argument to capture a review copy. The
default export is 2400 by 912 pixels. Inspect it at README desktop and mobile
widths, then capture a second copy and compare it with the tracked PNG.

For a PR with an inline preview, link to the PNG at the pushed commit so the
description shows the proposed artwork before merge.
