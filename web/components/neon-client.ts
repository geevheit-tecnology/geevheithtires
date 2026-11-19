"use client";
import { createAuthClient } from "@neondatabase/auth/next";
export const neonClient = createAuthClient();
export const neonEnabled = process.env.NEXT_PUBLIC_NEON_AUTH_ENABLED === "true";
