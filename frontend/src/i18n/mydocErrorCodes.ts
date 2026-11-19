/**
 * MyDoc API error codes (stable wire format).
 * Keys under mydoc.errors.* in locale files must stay in sync with these codes.
 * Referenced here so localeKeyAudit keeps the i18n entries.
 */
export const MYDOC_ERROR_CODES = [
  'MYDOC_DUPLICATE_FOLDER',
  'MYDOC_DUPLICATE_FILE',
  'MYDOC_DUPLICATE_NAME',
  'MYDOC_NOT_FOUND',
  'MYDOC_FORBIDDEN',
  'MYDOC_NOT_OWNER',
  'MYDOC_MOVE_CYCLE',
  'MYDOC_NOT_TEXT',
  'MYDOC_INVALID_INPUT',
  'MYDOC_MACHINE_CALLER',
] as const

export type MyDocErrorCode = (typeof MYDOC_ERROR_CODES)[number]

/** i18n key path for a MyDoc error code, e.g. mydoc.errors.MYDOC_DUPLICATE_FOLDER */
export function mydocErrorI18nKey(code: string): string {
  return `mydoc.errors.${code}`
}
