"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import {
  ArrowUpRight,
  ArrowRight,
  ShieldCheck,
  ScanLine,
  Activity,
  ChevronRight,
  Truck,
  Gauge,
  Check,
  Menu,
  X,
  CircleDot,
  BarChart3,
} from "lucide-react";
import { Brand } from "./brand";
import { TireMap, Tire } from "./tire-map";
const positions = [
  "1EE",
  "1DE",
  "2EE",
  "2EI",
  "2DI",
  "2DE",
  "3EE",
  "3EI",
  "3DI",
  "3DE",
  "STEP1",
];
const demo: Tire[] = positions.map((position, i) => ({
  id: String(i),
  position,
  code: `P00${123 + i}`,
  tread_mm: i === 4 ? 2.4 : i === 7 ? 4.2 : 8.5,
  status: i === 4 ? "critico" : i === 7 ? "alerta" : "ok",
}));
export default function Home() {
  const landingRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
    const elements = Array.from(
      landingRef.current?.querySelectorAll<HTMLElement>(
        ".section-heading, .center-heading, .feature, .calculator, .plan, .faq > div, .cta",
      ) || [],
    );
    const played = new WeakSet<Element>();
    const animations = new Set<Animation>();
    let observer: IntersectionObserver | undefined;
    const configure = () => {
      observer?.disconnect();
      if (preference.matches) {
        animations.forEach((animation) => animation.cancel());
        animations.clear();
        return;
      }
      if (!("IntersectionObserver" in window)) return;
      observer = new IntersectionObserver(
        (entries) => {
          entries.forEach((entry) => {
            if (!entry.isIntersecting || played.has(entry.target)) return;
            played.add(entry.target);
            observer?.unobserve(entry.target);
            const index = elements.indexOf(entry.target as HTMLElement);
            const animation = entry.target.animate(
              [
                { opacity: 0, transform: "translate3d(0, 24px, 0)" },
                { opacity: 1, transform: "translate3d(0, 0, 0)" },
              ],
              {
                duration: 680,
                delay: (index % 3) * 70,
                easing: "cubic-bezier(.22,1,.36,1)",
                fill: "backwards",
              },
            );
            animations.add(animation);
            animation.onfinish = () => animations.delete(animation);
          });
        },
        { threshold: 0.12 },
      );
      elements.forEach((element) => {
        if (!played.has(element)) observer?.observe(element);
      });
    };
    configure();
    preference.addEventListener("change", configure);
    return () => {
      observer?.disconnect();
      preference.removeEventListener("change", configure);
      animations.forEach((animation) => animation.cancel());
    };
  }, []);
  const [menu, setMenu] = useState(false);
  const [selected, setSelected] = useState<Tire>(demo[4]);
  const [fleet, setFleet] = useState(30);
  const [spend, setSpend] = useState(180000);
  const [reduction, setReduction] = useState(8);
  const money = (n: number) =>
    n.toLocaleString("pt-BR", {
      style: "currency",
      currency: "BRL",
      maximumFractionDigits: 0,
    });
  return (
    <>
      <header className="site-header">
        <div className="container nav">
          <Brand />
          <button
            className="menu-toggle"
            onClick={() => setMenu(!menu)}
            aria-label={menu ? "Fechar menu" : "Abrir menu"}
          >
            {menu ? <X /> : <Menu />}
          </button>
          <nav className={menu ? "open" : ""}>
            <a href="#plataforma" onClick={() => setMenu(false)}>
              A plataforma
            </a>
            <a href="#economia" onClick={() => setMenu(false)}>
              Sua economia
            </a>
            <a href="#planos" onClick={() => setMenu(false)}>
              Planos
            </a>
          </nav>
          <div className="nav-actions">
            <Link href="/login">
              Entrar <ArrowUpRight size={15} />
            </Link>
            <Link className="button small" href="/cadastro">
              Começar agora <ArrowRight size={16} />
            </Link>
          </div>
        </div>
      </header>
      <main ref={landingRef}>
        <section className="hero">
          <div className="container hero-grid">
            <div className="hero-copy">
              <div className="eyebrow">
                <span />
                GESTÃO DE PNEUS PARA QUEM MOVE O BRASIL
              </div>
              <h1>
                Mais estrada.{" "}
                <br />
                Menos custos.{" "}
                <br />
                <em>Cada pneu conta.</em>
              </h1>
              <p>
                Da primeira instalação à última recapagem, tenha clareza sobre
                cada pneu da sua frota. Transforme inspeções em decisões
                melhores.
              </p>
              <div className="hero-buttons">
                <Link href="/cadastro" className="button">
                  Começar teste de 14 dias <ArrowUpRight size={19} />
                </Link>
                <a className="text-link" href="#plataforma">
                  Conhecer a plataforma <ChevronRight size={17} />
                </a>
              </div>
              <div className="hero-notes">
                <ShieldCheck size={16} />
                Sem cartão de crédito <span>•</span> Feito para sua operação
              </div>
            </div>
            <div className="product-preview">
              <div className="preview-toolbar">
                <div>
                  <span className="preview-symbol">
                    <CircleDot size={18} />
                  </span>
                  <b>Visão da frota</b>
                </div>
                <span className="live-label">Exemplo ilustrativo</span>
              </div>
              <div className="preview-content">
                <div className="preview-info">
                  <span className="label">MAPA DE PNEUS</span>
                  <h3>
                    ABC-1234 <span>6×2</span>
                  </h3>
                  <p>Cavalo mecânico · 184.250 km</p>
                </div>
                <TireMap
                  positions={positions}
                  tires={demo}
                  onSelect={setSelected}
                />
                <div
                  key={selected.id}
                  className={`preview-alert ${selected.status}`}
                >
                  <Activity size={20} />
                  <div>
                    <b>
                      {selected.code} · {selected.position}
                    </b>
                    <p>
                      {selected.status === "critico"
                        ? "Sulco crítico. Planeje uma intervenção imediata."
                        : selected.status === "alerta"
                          ? "Atenção ao desgaste. Planeje a próxima avaliação."
                          : "Medição dentro da referência de sulco."}
                    </p>
                  </div>
                  <strong>
                    {selected.tread_mm.toLocaleString("pt-BR")} <small>mm</small>
                  </strong>
                </div>
              </div>
              <div className="preview-bottom">
                <span>
                  <i className="dot ok" /> Toque nos pneus para explorar o mapa
                </span>
                <ScanLine size={17} />
              </div>
            </div>
          </div>
          <div className="container hero-strip">
            <span>DO PÁTIO À GESTÃO</span>
            <span>
              <Truck />
              Frota sob controle
            </span>
            <span>
              <ScanLine />
              Inspeção simplificada
            </span>
            <span>
              <Gauge />
              Custo por quilômetro
            </span>
            <span>
              <ShieldCheck />
              Dados por empresa
            </span>
          </div>
        </section>
        <section id="plataforma" className="section container">
          <div className="section-heading">
            <div>
              <div className="eyebrow dark">
                UMA OPERAÇÃO. UMA VISÃO COMPLETA.
              </div>
              <h2>
                O pneu deixa sinais.{" "}
                <br />
                Sua gestão precisa enxergar.
              </h2>
            </div>
            <p>
              Site, painel e aplicativo conectados à mesma operação. Informação
              onde sua equipe precisa.
            </p>
          </div>
          <div className="feature-grid">
            <article className="feature">
              <span className="feature-number">01 / VISIBILIDADE</span>
              <CircleDot />
              <h3>
                Saiba onde cada{" "}
                <br />
                pneu está.
              </h3>
              <p>
                Mapa por veículo e posição, medições de sulco e alertas para
                priorizar a atenção da equipe.
              </p>
              <a href="/cadastro">
                Organizar minha frota <ArrowUpRight size={18} />
              </a>
            </article>
            <article className="feature">
              <span className="feature-number">02 / OPERAÇÃO</span>
              <ScanLine />
              <h3>
                Do pátio direto{" "}
                <br />
                para a decisão.
              </h3>
              <p>
                Registre inspeções e acompanhe o histórico. O aplicativo
                compartilha a mesma conta do painel.
              </p>
              <a href="/cadastro">
                Conhecer o fluxo <ArrowUpRight size={18} />
              </a>
            </article>
            <article className="feature feature-navy">
              <span className="feature-number">03 / EVOLUÇÃO</span>
              <BarChart3 />
              <h3>
                Construa uma gestão{" "}
                <br />
                guiada por dados.
              </h3>
              <p>
                Acompanhe o estoque, as movimentações e a manutenção dos pneus.
                O cálculo do custo por quilômetro e os recursos de inteligência
                artificial estão previstos para as próximas etapas.
              </p>
              <a href="#planos">
                Ver os planos previstos <ArrowUpRight size={18} />
              </a>
            </article>
          </div>
        </section>
        <section className="savings-section" id="economia">
          <div className="container savings-grid">
            <div>
              <div className="eyebrow dark">COLOQUE NA PONTA DO LÁPIS</div>
              <h2>
                Pequenas melhorias.{" "}
                <br />
                Mais economia.
              </h2>
              <p>
                Simule o impacto de reduzir o gasto anual com pneus. Ajuste os
                valores à realidade da sua transportadora.
              </p>
              <span className="caption">
                Simulação financeira, sem garantia de resultado.
              </span>
            </div>
            <div className="calculator">
              <label>
                Veículos na frota <strong>{fleet}</strong>
                <input
                  aria-label="Veículos na frota"
                  type="range"
                  min="5"
                  max="200"
                  value={fleet}
                  onChange={(e) => {
                    setFleet(+e.target.value);
                    setSpend(+e.target.value * 6000);
                  }}
                />
              </label>
              <label>
                Gasto anual com pneus (R$)
                <input
                  type="number"
                  min="0"
                  max="1000000000"
                  value={spend}
                  onChange={(e) =>
                    setSpend(Math.max(0, Math.min(1e9, +e.target.value)))
                  }
                />
              </label>
              <label>
                Redução estimada <strong>{reduction}%</strong>
                <input
                  aria-label="Redução estimada"
                  type="range"
                  min="1"
                  max="20"
                  value={reduction}
                  onChange={(e) => setReduction(+e.target.value)}
                />
              </label>
              <div className="calculation-result">
                <span>ECONOMIA ANUAL SIMULADA</span>
                <strong>{money((spend * reduction) / 100)}</strong>
                <small>
                  {money((spend * reduction) / 100 / 12)} por mês para investir
                  na sua operação
                </small>
              </div>
            </div>
          </div>
        </section>
        <section id="planos" className="section container">
          <div className="center-heading">
            <div className="eyebrow dark">CRESÇA NO SEU RITMO</div>
            <h2>Um plano para cada jornada.</h2>
            <p>
              Valores em definição. Os preços exibidos são provisórios e serão
              atualizados antes da contratação. Os recursos futuros estão
              identificados em cada plano.
            </p>
          </div>
          <div className="plans">
            {[
              {
                name: "Starter",
                price: "0,00",
                desc: "O primeiro passo para organizar sua frota.",
                items: [
                  "Até 15 veículos",
                  "3 usuários",
                  "Estoque e ciclo de vida",
                  "Mapa de pneus e inspeções",
                  "Alertas automáticos",
                ],
              },
              {
                name: "Pro",
                price: "0,00",
                desc: "Mais profundidade para sua operação.",
                items: [
                  "Até 60 veículos e 10 usuários",
                  "Gestão de estoque e ciclo de vida",
                  "Custo por km, IA e relatórios (previstos)",
                  "Gestão de até 3 filiais (prevista)",
                ],
              },
              {
                name: "Enterprise",
                price: "0,00",
                desc: "Para operações que precisam ir além.",
                items: [
                  "Limites sob consulta",
                  "Integrações (previstas)",
                  "API e webhooks (previstos)",
                  "Atendimento a combinar",
                ],
              },
            ].map((p, i) => (
              <article
                className={`plan ${i === 1 ? "featured" : ""}`}
                key={p.name}
              >
                {i === 1 && (
                  <span className="plan-tag">PARA FROTAS EM CRESCIMENTO</span>
                )}
                <span className="label">{p.name}</span>
                <p>{p.desc}</p>
                <div className="price">
                  <span>R$</span> {p.price}
                  <small>{" / mês"}</small>
                </div>
                <Link
                  href="/cadastro"
                  className={`button ${i === 1 ? "" : "outline"}`}
                >
                  Experimentar a plataforma <ArrowUpRight size={16} />
                </Link>
                <ul>
                  {p.items.map((s) => (
                    <li key={s}>
                      <Check size={16} />
                      {s}
                    </li>
                  ))}
                </ul>
              </article>
            ))}
          </div>
        </section>
        <section className="section faq container">
          <div>
            <div className="eyebrow dark">SEM DÚVIDAS NA ESTRADA</div>
            <h2>
              Antes de dar{" "}
              <br />a partida.
            </h2>
          </div>
          <div>
            {[
              [
                "Preciso cadastrar cartão para começar?",
                "Não. O cadastro inicia um período de avaliação de 14 dias. A contratação e a cobrança serão disponibilizadas na etapa comercial.",
              ],
              [
                "Minha equipe usa a mesma conta da empresa?",
                "Cada pessoa tem seu próprio login e perfil, vinculado à empresa. O painel e o aplicativo usam a mesma API.",
              ],
              [
                "Os dados de outras empresas ficam separados?",
                "O backend vincula os registros à empresa autenticada e o banco utiliza políticas de isolamento por empresa.",
              ],
              [
                "Todos os recursos dos planos já estão disponíveis?",
                "O controle de estoque, o ciclo de vida dos pneus, as inspeções e os alertas internos estão disponíveis no site. Os pagamentos ainda precisam ser ativados. Os recursos indicados como previstos e o funcionamento completo sem internet serão disponibilizados em etapas futuras.",
              ],
            ].map(([q, a]) => (
              <details key={q}>
                <summary>
                  {q}
                  <span>+</span>
                </summary>
                <p>{a}</p>
              </details>
            ))}
          </div>
        </section>
        <section className="cta container">
          <div>
            <div className="eyebrow">SUA PRÓXIMA ROTA COMEÇA AQUI</div>
            <h2>
              Leve sua gestão{" "}
              <br />
              mais longe.
            </h2>
          </div>
          <Link href="/cadastro" className="button">
            Começar meu teste <ArrowUpRight size={21} />
          </Link>
        </section>
      </main>
      <footer className="container footer">
        <Brand />
        <span>Desenvolvido por 2026@ Geevheit Software Solution</span>
        <Link href="/termos">Termos de Uso</Link>
        <Link href="/privacidade">Privacidade</Link>
        <Link href="/login">
          Acessar minha conta <ArrowUpRight size={14} />
        </Link>
      </footer>
    </>
  );
}
