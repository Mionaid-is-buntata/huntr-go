# Follow-up plan: Sources “Assess” and URL test clarity

This plan validates that the **Assess** action on the Settings → Sources list is visible, honest about behavior, and that the API returns enough detail to debug failures. Use it after merging or deploying the related changes.

## Goal

- Users see **immediate feedback** when clicking **Assess** (scroll + messages), not a silent no-op.
- The UI and API **do not claim** to run the full scraper/parser or write to Error History for this action.
- Failed checks show **actionable detail** (e.g. HTTP status or request error), not only “not accessible.”

## Preconditions

- Changes live on the branch `fix/sources-assess-url-check-ux` (or merged to `main` and deployed to the Pi/Docker stack as you normally release).

## Steps

1. **Ship the code**
   - Push the branch, open a PR, get review, merge to `main`.
   - Deploy to finlay (or your target) using your usual path (e.g. Docker compose rebuild/restart for `huntr-web`).

2. **Smoke test in the browser (Settings tab)**
   - Open **Settings** (Sources panel).
   - Click **Assess** on a source **low on a long list** (scroll position matters).
   - Confirm: the result block scrolls into view, loading text appears, then success or failure copy.
   - Confirm: the **Add New Source** area shows a short **source-message** line during/after the check.

3. **API check (optional but precise)**
   - `POST /api/sources/test` with `{"url":"https://example.com","dynamic":false}`.
   - Expect JSON including `message`, `status_code` (or `0` on transport error), `note` explaining reachability-only scope.

4. **Failure-mode review**
   - If many live sites return **403** to the server’s GET, the check may “fail” even when the site works in a browser. That is expected for this design; treat it as **signal**, not proof the source is bad.
   - Real scraper/parser outcomes remain in **Status → Error History** and scraper logs, not in this endpoint.

## Success criteria

- No silent click: user always sees loading and a final message in-panel or via `source-message`.
- Copy says **URL check** / reachability, not “parser OK” for this button.
- API returns structured fields (`status_code`, `note`) for support debugging.

## If the plan does not work

Document symptoms and findings in `docs/sources-assess-progress.md` (see “Failure signals” there), then narrow down:

| Symptom | Check |
|--------|--------|
| Click does nothing | Browser console for JS errors; confirm `assessSourceParser` / `sources-test-results` exist on the page. |
| No scroll / message | Hard refresh; verify deployed template is the new `dashboard.html`. |
| Always fails with 403/4xx | Compare with `curl -I` from the same host; many boards block non-browser clients. |
| PR/deploy missing | Confirm `huntr-web` image or bind-mount includes updated `src/templates/dashboard.html`. |

## Out of scope (later work)

- Running the **actual** parser or Rod fetch from the web handler for Assess.
- Auto-updating Error History from Assess (would be a product decision).
