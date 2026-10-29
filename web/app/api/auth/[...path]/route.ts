import { NextRequest, NextResponse } from "next/server";
import { neonAuth } from "../../../../lib/neon-auth";
async function handle(request: NextRequest, context: {params: Promise<{path:string[]}>}) {
  if (!process.env.NEON_AUTH_BASE_URL || !process.env.NEON_AUTH_COOKIE_SECRET)
    return NextResponse.json(
      { message: "Neon Auth não configurado." },
      { status: 503 },
    );
  const handlers = neonAuth().handler();
  const method = request.method as keyof typeof handlers;
  return handlers[method](request, context);
}
export {
  handle as GET,
  handle as POST,
  handle as PUT,
  handle as PATCH,
  handle as DELETE,
};
