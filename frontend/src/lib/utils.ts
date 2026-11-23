/**
 * Extract a human-readable message from an unknown thrown value, falling back to
 * `fallback` when the value isn't an Error.
 */
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback
}
