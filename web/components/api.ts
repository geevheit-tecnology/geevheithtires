export class ApiError extends Error {constructor(message:string,public status:number){super(message);this.name="ApiError";}}
export async function api(path: string, body?: unknown, method?: string) {
  const res = await fetch(`/api/backend/${path}`, {
    method: method || (body ? "POST" : "GET"),
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json();
  if (!res.ok)
    throw new ApiError(data.title || "Não foi possível concluir a solicitação.",res.status);
  return data;
}
