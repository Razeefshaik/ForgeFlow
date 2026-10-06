import { useQuery } from "@tanstack/react-query";
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
      .catch(() => ({ error: "Request failed" }))) as { error?: string };
    throw new Error(detail.error ?? `Request failed (${response.status})`);
  }
  return response.json() as Promise<T>;
}
export function useAPI<T>(path: string) {
  return useQuery({ queryKey: [path], queryFn: () => request<T>(path) });
}
