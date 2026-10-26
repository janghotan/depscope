import { checkApiHealth } from "@/lib/api-client";
import { jsonResponse } from "@/lib/http";

export const dynamic = "force-dynamic";

export async function GET(request: Request): Promise<Response> {
  const healthy = await checkApiHealth({ signal: request.signal });
  return jsonResponse(
    { status: healthy ? "ok" : "unavailable" },
    healthy ? 200 : 503,
  );
}
