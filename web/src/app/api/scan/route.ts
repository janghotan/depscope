import { requestScan } from "@/lib/api-client";
import {
  BodyLimitError,
  InvalidJsonError,
  hasJsonContentType,
  jsonError,
  jsonResponse,
  parseJsonObject,
  readStreamCapped,
} from "@/lib/http";
import { parsePackageInput } from "@/lib/request";

export const dynamic = "force-dynamic";

const MAX_REQUEST_BYTES = 16 * 1024;

export async function POST(request: Request): Promise<Response> {
  if (new URL(request.url).search !== "") {
    return jsonError(400, "invalid_request", "query parameters are not accepted");
  }
  if (!hasJsonContentType(request)) {
    return jsonError(
      415,
      "unsupported_media_type",
      "content type must be application/json",
    );
  }

  let text: string;
  try {
    text = await readStreamCapped(request.body, MAX_REQUEST_BYTES);
  } catch (error) {
    if (error instanceof BodyLimitError) {
      return jsonError(
        413,
        "request_too_large",
        "request body exceeds the 16 KiB limit",
      );
    }
    return jsonError(400, "invalid_json", "request body must contain valid JSON");
  }

  let body: Record<string, unknown>;
  try {
    body = parseJsonObject(text);
  } catch (error) {
    const message =
      error instanceof InvalidJsonError
        ? error.message
        : "request body must contain valid JSON";
    return jsonError(400, "invalid_json", message);
  }

  const parsed = parsePackageInput(body);
  if (!parsed.ok) {
    return jsonError(400, parsed.issue.code, parsed.issue.message);
  }

  const result = await requestScan(parsed.value, { signal: request.signal });
  if (!result.ok) {
    return jsonError(
      result.error.status,
      result.error.code,
      result.error.message,
      result.error.retryAfter
        ? { "Retry-After": result.error.retryAfter }
        : undefined,
    );
  }
  return jsonResponse(result.value);
}
