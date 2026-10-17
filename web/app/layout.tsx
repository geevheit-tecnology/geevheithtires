import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = {title:"GeevheitTire | Cada pneu conta.",description:"Gestão de pneus para transportadoras. Controle sua frota, acompanhe inspeções e transforme dados em decisões."};
export default function Layout({children}:{children:React.ReactNode}){return <html lang="pt-BR"><body>{children}</body></html>}
