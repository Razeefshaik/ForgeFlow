import { useQuery } from "@tanstack/react-query";

export class APIError extends Error {
  constructor(message: string, public status: number, public retryAt?: string) {
    super(message);
    this.name = "APIError";
  }
}

export async function request<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(
    "/api" + path,
    body === undefined
      ? undefined
      : {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
  );
  if (!response.ok) {
    const detail = (await response
      .json()
      .catch(() => ({ error: "Request failed" }))) as { error?: string; retry_at?: string };
    const original = detail.error ?? `Request failed (${response.status})`;
    // Accept the existing server contract as well as a structured retry timestamp.
    const retryAt = detail.retry_at ?? original.match(/retry after (\d{4}-\d{2}-\d{2}T[\d:.]+Z)/i)?.[1];
    const retryDate = retryAt ? new Date(retryAt) : null;
    const message = retryDate && !Number.isNaN(retryDate.getTime())
      ? original.replace(retryAt!, retryDate.toLocaleString(undefined, { timeZoneName: "short" }))
      : original;
    throw new APIError(message, response.status, retryAt);
  }
  return response.json() as Promise<T>;
}
export function useAPI<T>(path: string) {
  return useQuery({ queryKey: [path], queryFn: () => request<T>(path) });
}
