import { readFileSync } from "node:fs";
// Local/container regression check. Vercel moves these files in its adapter;
// that environment uses build:vercel instead of the local artifact check.
const checks = [
  ["index.html", "Mais estrada."],
  ["app.html", "Olá,"],
];
for (const [file, expected] of checks) {
  const html = readFileSync(`.next/server/app/${file}`, "utf8");
  const heading = html.match(/<h1[^>]*>([\s\S]*?)<\/h1>/)?.[1] || "";
  const text = heading.replace(/<[^>]*>/g, "");
  if (!text.includes(expected)) {
    throw new Error(
      `Rota incorreta em ${file}: esperado "${expected}", recebido "${text}"`,
    );
  }
}
console.log("Rotas validadas: landing em / e painel em /app.");
