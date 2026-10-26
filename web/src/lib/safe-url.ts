/**
 * 只返回可用于链接的 http(s) URL。
 * 远端报告中的字符串不可信，必须拒绝 javascript:、data: 以及带凭据的地址。
 */
export function safeHttpUrl(value: string | undefined | null): string | null {
  if (!value) {
    return null;
  }
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      return null;
    }
    if (!url.hostname || url.username || url.password) {
      return null;
    }
    return url.toString();
  } catch {
    return null;
  }
}
