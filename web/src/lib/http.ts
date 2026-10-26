export class BodyLimitError extends Error {
  constructor() {
    super("Request body exceeds the configured limit.");
    this.name = "BodyLimitError";
  }
}

export class InvalidJsonError extends Error {
  constructor(message = "Request body must contain valid JSON.") {
    super(message);
    this.name = "InvalidJsonError";
  }
}

/** 以字节数限制读取流，避免把不受信任的响应整体载入内存。 */
export async function readStreamCapped(
  stream: ReadableStream<Uint8Array> | null,
  maxBytes: number,
): Promise<string> {
  if (!stream) {
    return "";
  }
  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      total += value.byteLength;
      if (total > maxBytes) {
        await reader.cancel();
        throw new BodyLimitError();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const merged = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.byteLength;
  }

  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(merged);
  } catch {
    throw new InvalidJsonError("Request body must be UTF-8 encoded JSON.");
  }
}

/** 解析一个且仅一个 JSON 对象；数组、标量与尾随内容都会被拒绝。 */
export function parseJsonObject(text: string): Record<string, unknown> {
  if (text.trim() === "") {
    throw new InvalidJsonError("Request body must not be empty.");
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new InvalidJsonError("Request body must contain valid JSON.");
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    throw new InvalidJsonError("Request body must be a JSON object.");
  }
  return parsed as Record<string, unknown>;
}

export function hasJsonContentType(request: Request): boolean {
  const header = request.headers.get("content-type");
  if (!header) {
    return false;
  }
  const [mediaType] = header.split(";");
  return mediaType.trim().toLowerCase() === "application/json";
}

const BASE_HEADERS = {
  "Cache-Control": "no-store",
  "X-Content-Type-Options": "nosniff",
} as const;

export function jsonResponse(
  value: unknown,
  status = 200,
  extraHeaders?: HeadersInit,
): Response {
  const headers = new Headers(BASE_HEADERS);
  if (extraHeaders) {
    new Headers(extraHeaders).forEach((headerValue, key) => {
      headers.set(key, headerValue);
    });
  }
  headers.set("Content-Type", "application/json; charset=utf-8");
  return new Response(JSON.stringify(value), { status, headers });
}

export function jsonError(
  status: number,
  code: string,
  message: string,
  extraHeaders?: HeadersInit,
): Response {
  return jsonResponse({ error: { code, message } }, status, extraHeaders);
}
