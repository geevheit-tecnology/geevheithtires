"use client";
export type Tire = {
  id: string;
  code: string;
  tread_mm: number;
  status: string;
  position?: string;
  brand?: string;
  size?: string;
  state?: string;
  vehicle_id?: string;
  version?: number;
  stock_location?: string;
};
export function TireMap({
  positions,
  tires,
  onSelect,
}: {
  positions: string[];
  tires: Tire[];
  onSelect?: (t: Tire) => void;
}) {
  const axles = [
    ...new Set(positions.filter((p) => !p.startsWith("STEP")).map((p) => p[0])),
  ];
  function tire(p: string) {
    const t = tires.find((t) => t.position === p);
    return (
      <button
        key={p}
        type="button"
        className={`tire ${t?.status || "empty"}`}
        disabled={!t || !onSelect}
        onClick={() => t && onSelect?.(t)}
        aria-label={`${p}: ${t ? `${t.code}, ${t.tread_mm} milímetros` : "sem pneu"}`}
      >
        <span>{p}</span>
        <b>{t?.code || "—"}</b>
        <small>{t ? `${Number(t.tread_mm).toFixed(1)} mm` : "Vazio"}</small>
      </button>
    );
  }
  return (
    <div className="vehicle-map">
      <div className="front">↑ FRENTE DO VEÍCULO</div>
      <div className="chassis">
        {axles.map((a) => (
          <div className="axle" key={a}>
            <div className="wheel-pair">
              {positions.filter((p) => p[0] === a && p[1] === "E").map(tire)}
            </div>
            <span className="axle-label">{a}</span>
            <div className="wheel-pair">
              {positions.filter((p) => p[0] === a && p[1] === "D").map(tire)}
            </div>
          </div>
        ))}
      </div>
      <div className="spares">
        {positions.filter((p) => p.startsWith("STEP")).map(tire)}
      </div>
      <div className="legend">
        <span>
          <i className="dot ok" />
          OK
        </span>
        <span>
          <i className="dot alerta" />
          Alerta
        </span>
        <span>
          <i className="dot critico" />
          Crítico
        </span>
        <span>
          <i className="dot empty" />
          Sem pneu
        </span>
      </div>
    </div>
  );
}
