import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";
const publicPaths = new Set([
  "auth/register",
  "auth/neon",
  "auth/login",
  "auth/recover",
  "auth/accept",
]);
async function proxy(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const path = (await params).path.join("/");
  const isPublic = publicPaths.has(path) || /^registration\/(?:cnpj\/[A-Z0-9]{14}|cep\/[0-9]{8})$/.test(path);
  if (!/^[a-zA-Z0-9\-/]+$/.test(path))
    return NextResponse.json({ title: "Caminho inválido." }, { status: 400 });
  if (req.method !== "GET") {
    const origin = req.headers.get("origin");
    const host = req.headers.get("host");
    const protocol =
      req.headers.get("x-forwarded-proto") ||
      req.nextUrl.protocol.replace(":", "");
    if (!origin || !host || origin !== `${protocol}://${host}`)
      return NextResponse.json(
        { title: "Origem não permitida." },
        { status: 403 },
      );
  }
  const base = process.env.API_URL;
  if (!base)
    return NextResponse.json(
      {
        title:
          "A API ainda não foi configurada neste ambiente. O cadastro estará disponível após a conexão do backend.",
      },
      { status: 503 },
    );
  const jar = await cookies();
  const options = {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path: "/",
  };
  let access = jar.get("gt_access")?.value;
  const refresh = jar.get("gt_refresh")?.value;
  let body: string | undefined =
    req.method === "GET" ? undefined : await req.text();
  if (body && body.length > 1_000_000)
    return NextResponse.json(
      { title: "Dados muito grandes." },
      { status: 413 },
    );
  if (path === "auth/logout")
    body = JSON.stringify({ refresh_token: refresh || "" });
  const send = () =>
    fetch(`${base}/api/v1/${path}${req.nextUrl.search}`, {
      method: req.method,
      headers: {
        "Content-Type": "application/json",
        ...(access ? { Authorization: `Bearer ${access}` } : {}),
      },
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(20000),
    });
  try {
    let response = await send();
    if (
      response.status === 401 &&
      refresh &&
      !isPublic &&
      path !== "auth/logout"
    ) {
      const rr = await fetch(`${base}/api/v1/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
        cache: "no-store",
        signal: AbortSignal.timeout(10000),
      });
      if (rr.ok) {
        const session = await rr.json();
        access = session.access_token;
        jar.set("gt_access", access!, { ...options, maxAge: 900 });
        jar.set("gt_refresh", session.refresh_token, {
          ...options,
          maxAge: 2592000,
        });
        response = await send();
      } else {
        jar.delete("gt_access");
        jar.delete("gt_refresh");
      }
    }
    const result = await response.json();
    if (response.ok && result.access_token) {
      jar.set("gt_access", result.access_token, { ...options, maxAge: 900 });
      jar.set("gt_refresh", result.refresh_token, {
        ...options,
        maxAge: 2592000,
      });
      delete result.access_token;
      delete result.refresh_token;
    }
    if (path === "auth/logout") {
      jar.delete("gt_access");
      jar.delete("gt_refresh");
    }
    return NextResponse.json(result, {
      status: response.status,
      headers: { "Cache-Control": "no-store" },
    });
  } catch {
    return NextResponse.json(
      {
        title:
          "Não foi possível conectar ao servidor. Tente novamente em instantes.",
      },
      { status: 503 },
    );
  }
}
export { proxy as GET, proxy as POST, proxy as PATCH };
