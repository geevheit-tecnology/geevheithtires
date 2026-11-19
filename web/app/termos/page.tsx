import LegalDocument from "../../components/legal-document";
export const metadata = { title: "Termos de Uso | GeevheitTire" };
export default function TermsPage() {
  return <LegalDocument title="Termos de Uso">
    <section><h2>Serviço e avaliação</h2><p>O GeevheitTire oferece ferramentas de gestão de pneus, veículos e inspeções. O cadastro inicia uma avaliação de 14 dias, sujeita aos limites exibidos no painel. Recursos identificados como roadmap ainda não estão disponíveis.</p></section>
    <section><h2>Conta e responsabilidades</h2><p>Cada pessoa deve utilizar seu próprio acesso. O proprietário da empresa administra os convites e os perfis da equipe. As informações e inspeções registradas devem refletir a operação real. Os alertas do sistema apoiam a gestão; decisões de segurança operacional exigem avaliação técnica adequada.</p></section>
    <section><h2>Planos e pagamentos</h2><p>A disponibilidade de contratação e os limites constam em Plano e cobrança. A implementação atual utiliza mensalidades por link quando o gateway está habilitado, sem renovação automática. Não informe dados de cartão diretamente no GeevheitTire. Condições de contratação recorrente, cancelamento, reembolso e atendimento precisam ser definidas e revisadas antes da oferta comercial.</p></section>
    <section><h2>Encerramento e suporte</h2><p>Após o vencimento da avaliação ou do período pago, o acesso pode ficar limitado à consulta. Os procedimentos de exportação, exclusão, retenção e encerramento de conta, assim como o canal oficial de suporte, precisam ser concluídos antes do lançamento.</p></section>
    <section><h2>Identificação do fornecedor</h2><p>Pendente de preenchimento pelo responsável: razão social, CNPJ, endereço, contato, condições de disponibilidade e versão final deste documento.</p></section>
  </LegalDocument>;
}
