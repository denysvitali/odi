const MAX_LINES = 30

const COMPANY_KEYWORDS = [
  'ag',
  'gmbh',
  'sa',
  'sagl',
  'sàrl',
  'srl',
  'groupe',
  'group',
  'holding',
  'insurance',
  'assicur',
  'bank',
  'club'
]

const COMPANY_STOP_PREFIXES = [
  'signore',
  'signora',
  'egregio',
  'dear',
  'cordiali',
  'ciao',
  'p.s',
  'grü',
  'p\.s\.'
]

function normalizeLine(line: string): string {
  return line
    .replace(/\s+/g, ' ')
    .replace(/^[\"'`]+|[\"'`,;:.]+$/g, '')
    .trim()
}

function splitTextLines(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map(normalizeLine)
    .filter((line) => line.length > 0)
}

function hasWords(line: string): boolean {
  return line.split(/\s+/).length >= 2
}

function allUppercaseWords(line: string): boolean {
  const words = line.split(/\s+/).filter(Boolean)
  if (words.length === 0) return false
  return words.every((word) => {
    const cleaned = word.replace(/[^\p{L}\p{N}'’.-]/gu, '')
    return cleaned.length > 0 && cleaned[0].toUpperCase() === cleaned[0]
  })
}

function hasAnyCompanyKeyword(line: string): boolean {
  const normalized = line.toLowerCase()
  return COMPANY_KEYWORDS.some((keyword) => normalized.includes(` ${keyword} `) || normalized.endsWith(` ${keyword}`) || normalized.startsWith(`${keyword} `))
}

function isAddressOrContactLine(line: string): boolean {
  const lowered = line.toLowerCase()
  if (/[0-9]+/.test(line) && /(strasse|str\. |str\.|str |avenue|street|road|via|rue|platz|casella|zip|tel|telefon|phone|fax|www\.|http|\S+@\S+)/i.test(line)) {
    return true
  }
  return /\d{5}|\d{4,}/.test(line)
}

function isGreetingOrLabel(line: string): boolean {
  const lowered = line.toLowerCase()
  return COMPANY_STOP_PREFIXES.some((prefix) => lowered.startsWith(prefix)) || /^(to|from|betreff|subject|date|datum):/i.test(lowered)
}

export function extractCompanyFromText(text: string): string {
  const lines = splitTextLines(text)

  for (const line of lines.slice(0, MAX_LINES)) {
    if (!hasWords(line) || isGreetingOrLabel(line) || isAddressOrContactLine(line) || line.length > 60) {
      continue
    }

    if (hasAnyCompanyKeyword(line) && allUppercaseWords(line)) {
      return line
    }
  }

  for (const line of lines.slice(0, MAX_LINES)) {
    const words = line.split(/\s+/)
    if (words.length < 3 || words.length > 7) {
      continue
    }
    if (line.length > 60 || isGreetingOrLabel(line) || isAddressOrContactLine(line) || !allUppercaseWords(line)) {
      continue
    }
    return line
  }

  return ''
}

export function extractTitleFromText(text: string): string {
  const lines = splitTextLines(text)

  for (const line of lines.slice(0, MAX_LINES)) {
    const wordsCount = line.split(/\s+/).length

    if (line.length < 16 || line.length > 140) {
      continue
    }
    if (wordsCount < 3 || wordsCount > 18) {
      continue
    }
    if (isGreetingOrLabel(line) || isAddressOrContactLine(line)) {
      continue
    }
    if (hasAnyCompanyKeyword(line) && allUppercaseWords(line)) {
      continue
    }
    if (!/[A-Za-zÀ-ÖØ-öø-ÿ]/.test(line)) {
      continue
    }
    if (/www\.|http|\S+@\S+/i.test(line)) {
      continue
    }

    return line
  }

  return ''
}
