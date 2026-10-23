"use client";
import { useState } from "react";
import { TireMap } from "./tire-map";

export type VehicleConfiguration = { id: string; category: string; name: string; positions: string[] };

export default function VehicleConfigurationFields({ configurations }: { configurations: VehicleConfiguration[] }) {
  const categories = [...new Set(configurations.map(c => c.category))];
  const [category, setCategory] = useState("");
  const [selected, setSelected] = useState("");
  const options = configurations.filter(c => c.category === category);
  const configuration = options.find(c => c.id === selected);
  const positions = configuration?.positions || [];
  const rolling = positions.filter(p => !p.startsWith("STEP"));
  const axles = new Set(rolling.map(p => p.match(/^\d+/)?.[0])).size;
  return <>
    <label>Tipo de veículo
      <select required value={category} onChange={e => { const value = e.target.value; setCategory(value); setSelected(configurations.find(c => c.category === value)?.id || ""); }}>
        <option value="">Selecione o tipo</option>
        {categories.map(c => <option key={c} value={c}>{c}</option>)}
      </select>
    </label>
    <label>Configuração de eixos e rodado
      <select name="template_id" required disabled={!category} value={selected} onChange={e => setSelected(e.target.value)}>
        <option value="">Selecione a configuração</option>
        {options.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
      </select>
    </label>
    <p>Confira a configuração real do veículo. O rodado pode variar dentro do mesmo tipo.</p>
    {configuration && <div aria-live="polite">
      <p><b>{axles} eixos · {rolling.length} pneus em rodagem · {positions.length - rolling.length} estepe(s)</b></p>
      <TireMap positions={positions} tires={[]} />
    </div>}
  </>;
}
