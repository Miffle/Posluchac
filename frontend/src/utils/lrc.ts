export interface LyricLine {
  time: number // in seconds
  text: string
}

/**
 * parseLRC parses standard and multi-timestamp LRC format into sorted LyricLine entries.
 * Handles [mm:ss.xx] timestamps and filters metadata tags like [ti:], [ar:], etc.
 */
export function parseLRC(lrc: string): LyricLine[] {
  if (!lrc || typeof lrc !== 'string') return []

  const lines = lrc.split(/\r?\n/)
  const result: LyricLine[] = []
  // Matches [mm:ss] or [mm:ss.xx] or [mm:ss.xxx]
  const timeRegex = /\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]/g

  for (const rawLine of lines) {
    const trimmed = rawLine.trim()
    if (!trimmed) continue

    // Skip metadata headers like [ti:Title], [ar:Artist], [offset:0]
    if (/^\[(ti|ar|al|au|length|by|offset|re|ve):/i.test(trimmed)) {
      continue
    }

    const timestamps: number[] = []
    let match: RegExpExecArray | null
    timeRegex.lastIndex = 0

    while ((match = timeRegex.exec(trimmed)) !== null) {
      const minutes = parseInt(match[1], 10)
      const seconds = parseInt(match[2], 10)
      let fraction = 0
      if (match[3]) {
        const fracStr = match[3]
        if (fracStr.length === 1) {
          fraction = parseInt(fracStr, 10) / 10
        } else if (fracStr.length === 2) {
          fraction = parseInt(fracStr, 10) / 100
        } else {
          fraction = parseInt(fracStr, 10) / 1000
        }
      }
      timestamps.push(minutes * 60 + seconds + fraction)
    }

    if (timestamps.length === 0) continue

    // Extract text after removing all bracketed timestamp tags
    const text = trimmed.replace(/\[\d{1,2}:\d{2}(?:\.\d{1,3})?\]/g, '').trim()

    for (const time of timestamps) {
      result.push({ time, text })
    }
  }

  // Sort chronologically by timestamp
  result.sort((a, b) => a.time - b.time)
  return result
}
