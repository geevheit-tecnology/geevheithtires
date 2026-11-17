import type { Metadata, Viewport } from "next";
import "./globals.css";
import { PwaRegister } from "../components/pwa-register";
export const metadata: Metadata = {
  applicationName: "GeevheitTire",
  manifest: "/manifest.webmanifest",
  appleWebApp: { capable: true, title: "GeevheitTire", statusBarStyle: "black-translucent" },
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/icon.svg", type: "image/svg+xml" },
      { url: "/favicon-32.png", sizes: "32x32", type: "image/png" },
      { url: "/favicon-16.png", sizes: "16x16", type: "image/png" },
    ],
    apple: [{ url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" }],
  },
  title: "GeevheitTire | Cada pneu conta.",
  description:
    "Gestão de pneus para transportadoras. Controle sua frota, acompanhe inspeções e transforme dados em decisões.",
};
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: "#14201d",
};
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="pt-BR">
      <body>
        <PwaRegister />
        {children}
      </body>
    </html>
  );
}
