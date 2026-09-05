# ADR 0010: Safe browser previews from short-lived object URLs

**Status:** Accepted
**Date:** 2026-09-04

## Context

Eterealink stores user-supplied files in a private Cloud Storage bucket. Existing signed download URLs force an attachment disposition, which is appropriate for downloads but cannot support an embedded preview. Serving arbitrary uploaded content inline would let active formats such as SVG or HTML run in a browser context and would make the declared media type part of the security boundary.

Phase 6 requires useful previews for shared and authenticated files without introducing a preview worker or a second object copy.

## Decision

- Keep attachment download targets and preview targets separate. Preview targets use a short-lived signed URL with an inline disposition and an explicit response content type.
- Return a preview target only for a server-side allowlist: JPEG, PNG, GIF, WebP, AVIF, PDF, common browser-native video and audio formats, and text content.
- Render `text/*`, JSON, and XML as escaped text rather than browser-interpreted markup. Limit text previews to files no larger than 1 MiB.
- Exclude SVG and unsupported binary formats from inline previews. Their UI shows metadata, a generic file treatment, and the authorized download action.
- Render PDFs in a cross-origin, no-referrer frame so built-in browser PDF viewers can operate while the storage origin remains isolated from the application by the same-origin policy. Use native audio controls and place accessible Eterealink controls around the browser's native video element. The video controls expose ten-second skips, playback speed, volume, picture-in-picture, fullscreen, buffering and codec feedback, retry, auto-hide behavior, and locally remembered volume and speed. Video playback uses the original stored object and reports its decoded source resolution; no lower-resolution rendition is substituted.
- Apply the same preview policy to public file shares, files inside anonymous multi-file transfers, private library files, and files inherited through folder access.
- Do not generate thumbnails or transformed derivatives in this phase.

## Consequences

Preview URLs retain the authorization lifetime of their corresponding download URLs and never outlive an expiring share or anonymous transfer. The browser reads file bytes directly from Cloud Storage, so the API remains stateless and does not proxy file content.

Browser codec and PDF support may vary. Unsupported media, oversized text, and failed preview loads preserve the download path. Captions require associated subtitle-track metadata. Timeline thumbnails require frame extraction and generated assets. A selectable quality ladder requires transcoding and storing additional renditions. These capabilities, rich document conversion, and adaptive video remain candidates for a later asynchronous preview pipeline.
