"use client";
import { FormEvent, useState } from "react";
import { api } from "./api";
export default function VehicleOdometer({
  vehicle,
  onChange,
}: {
  vehicle: { id: string; plate: string; current_km: number };
  onChange: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setMessage("");
    const current_km = Number(new FormData(e.currentTarget).get("current_km"));
    try {
      await api(`vehicles/${vehicle.id}/odometer`, { current_km }, "PATCH");
      await onChange();
      setMessage("Quilometragem e alertas de manutenção atualizados.");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="odometer-form" onSubmit={submit}>
      <label>
        Odômetro de {vehicle.plate} (km)
        <input
          key={`${vehicle.id}-${vehicle.current_km}`}
          name="current_km"
          type="number"
          min={vehicle.current_km}
          max={2147483647}
          defaultValue={vehicle.current_km}
          required
        />
      </label>
      <button className="button small" disabled={busy}>
        {busy ? "Atualizando…" : "Atualizar km"}
      </button>
      {message && <p role="status">{message}</p>}
    </form>
  );
}
