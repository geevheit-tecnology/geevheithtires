import { createNeonAuth } from "@neondatabase/auth/next/server";
export function neonAuth() {
  const baseUrl = process.env.NEON_AUTH_BASE_URL;
  const secret = process.env.NEON_AUTH_COOKIE_SECRET;
  if (!baseUrl || !secret) throw new Error("Neon Auth não configurado.");
  return createNeonAuth({ baseUrl, cookies: { secret } });
}
