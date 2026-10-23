import Link from "next/link";

export default function LegalDocument({ title, children }: { title: string; children: React.ReactNode }) {
  return <main className="container section" style={{ maxWidth: 840 }}>
    <Link href="/">← GeevheitTire</Link>
    <h1>{title}</h1>
    <p className="info"><strong>Rascunho para revisão jurídica.</strong> Esta versão precisa da identificação da empresa responsável, do canal de atendimento e da aprovação jurídica antes do lançamento comercial.</p>
    <article style={{ display: "grid", gap: 20, marginTop: 24 }}>{children}</article>
    <nav aria-label="Documentos legais" style={{ display: "flex", flexWrap: "wrap", gap: 24, marginTop: 32 }}>
      <Link href="/termos">Termos de Uso</Link><Link href="/privacidade">Política de Privacidade</Link>
    </nav>
  </main>;
}
