/**
 * NDJSON (newline-delimited JSON) stream parser.
 * Parses a streaming HTTP Response into individual JSON objects.
 */

/**
 * Parse an NDJSON stream from a fetch Response into an async iterable of objects.
 * Each yielded value is one parsed JSON line from the stream.
 *
 * Uses explicit getReader() rather than `for await (chunk of body)` for
 * reliable behavior across runtimes (Bun's async iteration can skip chunks).
 *
 * Usage:
 *   const response = await fetch(url, { ... });
 *   for await (const event of parseNdjson(response)) {
 *     console.log(event.type, event);
 *   }
 */
export async function* parseNdjson<T = Record<string, unknown>>(
  response: Response,
  onRawLine?: (line: string) => void,
): AsyncIterable<T> {
  const body = response.body;
  if (!body) {
    onRawLine?.("[no body]");
    return;
  }

  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Process complete lines
      let newlineIdx: number;
      while ((newlineIdx = buffer.indexOf("\n")) !== -1) {
        const line = buffer.slice(0, newlineIdx).trim();
        buffer = buffer.slice(newlineIdx + 1);

        if (line.length === 0) continue;

        onRawLine?.(line);
        try {
          yield JSON.parse(line) as T;
        } catch {
          // Skip malformed lines — Notion occasionally sends empty or partial lines
        }
      }
    }
  } finally {
    reader.releaseLock();
  }

  // Flush remaining buffer (if no trailing newline)
  const remaining = buffer.trim();
  if (remaining.length > 0) {
    onRawLine?.(remaining);
    try {
      yield JSON.parse(remaining) as T;
    } catch {
      // Ignore trailing non-JSON
    }
  }
}
