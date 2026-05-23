/**
 * Lightweight logging helper used across the frontend.
 *
 * - `warn` / `error` always log (we want operators to see real failures even in prod).
 * - `info` / `debug` are silenced in production builds to keep the console clean.
 *
 * All messages are prefixed with `[odi]` so they are easy to grep in the browser console.
 */

const PREFIX = '[odi]'

function isProd(): boolean {
  try {
    return Boolean(import.meta.env?.PROD)
  } catch {
    return false
  }
}

export const logger = {
  debug(message: string, ...rest: unknown[]): void {
    if (isProd()) return
     
    console.debug(PREFIX, message, ...rest)
  },
  info(message: string, ...rest: unknown[]): void {
    if (isProd()) return
     
    console.info(PREFIX, message, ...rest)
  },
  warn(message: string, ...rest: unknown[]): void {
     
    console.warn(PREFIX, message, ...rest)
  },
  error(message: string, ...rest: unknown[]): void {
     
    console.error(PREFIX, message, ...rest)
  }
}

export type Logger = typeof logger
