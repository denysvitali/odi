import type { ClassValue } from 'clsx'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Combines class names using clsx and merges Tailwind classes using tailwind-merge.
 * Useful for conditional classes and avoiding conflicts.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}

/**
 * Extract a human-readable message from an unknown thrown value, falling back to
 * `fallback` when the value isn't an Error (ApiError extends Error, so it is
 * covered here too).
 */
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback
}
