import type { NextConfig } from "next";
import path from "node:path";
const config: NextConfig = {
  output: "standalone",
 outputFileTracingRoot: path.resolve(process.cwd()),
  poweredByHeader: false,
  experimental: { cpus: 1, staticGenerationMaxConcurrency: 1 },
};
export default config;
