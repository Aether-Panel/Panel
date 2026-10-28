/**
 * After the panel is updated its build files get new names, and a page opened before still
 * asks for the old ones. The import then fails ("Failed to fetch dynamically imported module")
 * although nothing is broken: loading the page again picks up the new files.
 */
const STALE_BUILD_ERROR = /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|Unable to preload CSS/i;
const RELOADED_AT_KEY = 'aether:stale-build-reload';

export function isStaleBuildError(error: unknown): boolean {
    const message = error instanceof Error ? error.message : String(error ?? '');
    return STALE_BUILD_ERROR.test(message);
}

/**
 * Reloads the page once. Returns false when it already did in the last minute, so a file that
 * is really missing shows its error instead of reloading forever.
 */
export function reloadForNewBuild(): boolean {
    try {
        const last = Number(sessionStorage.getItem(RELOADED_AT_KEY) || 0);
        if (Date.now() - last < 60_000) return false;
        sessionStorage.setItem(RELOADED_AT_KEY, String(Date.now()));
    } catch {
        // without storage the reload still happens once per error
    }
    window.location.reload();
    return true;
}
