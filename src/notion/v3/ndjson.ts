/**
 * NDJSON (newline-delimited JSON) stream parser.
 * Parses a streaming HTTP Response into individual JSON objects.
 */

/**
 * Parse an NDJSON stream from a fetch Response into an async iterable of objects.
 * Each yielded value is one parsed JSON line from the stream.
 *
 * Usage:
 *   const response = await fetch(url, { ... });
 *   for await (const event of parseNdjson(response)) {
 *     console.log(event.type, event);
 *   }
 */
export async function* parseNdjson<T = Record<string, unknown>>(
  response: Response,
): AsyncIterable<T> {
  const body = response.body;
  if (!body) return;

  const decoder = new TextDecoder();
  let buffer = "";

  for await (const chunk of body) {
    buffer += decoder.decode(chunk as Uint8Array, { stream: true });

    // Process complete lines
    let newlineIdx: number;
    while ((newlineIdx = buffer.indexOf("\n")) !== -1) {
      const line = buffer.slice(0, newlineIdx).trim();
      buffer = buffer.slice(newlineIdx + 1);

      if (line.length === 0) continue;

      try {
        yield JSON.parse(line) as T;
      } catch {
        // Skip malformed lines — Notion occasionally sends empty or partial lines
      }
    }
  }

  // Flush remaining buffer (if no trailing newline)
  const remaining = buffer.trim();
  if (remaining.length > 0) {
    try {
      yield JSON.parse(remaining) as T;
    } catch {
      // Ignore trailing non-JSON
    }
  }
}
