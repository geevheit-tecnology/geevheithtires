"use client";

export function TireLoader({
  label = "Sincronizando telemetria da frota…",
  sublabel = "Carregando sulcos, pressão e histórico",
  compact = false,
}: {
  label?: string;
  sublabel?: string;
  compact?: boolean;
}) {
  return (
    <div className={compact ? "tire-loader compact" : "tire-loader"} role="status" aria-live="polite">
      <div className="tire-loader-stage">
        <div className="tire-loader-pulse" />
        <svg
          className="tire-loader-wheel"
          viewBox="0 0 120 120"
          width={compact ? 64 : 96}
          height={compact ? 64 : 96}
          aria-hidden="true"
        >
          <defs>
            <linearGradient id="gt-rubber" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="#1c2a27" />
              <stop offset="50%" stopColor="#2c3f3a" />
              <stop offset="100%" stopColor="#121c19" />
            </linearGradient>
            <linearGradient id="gt-rim" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="#dce7df" />
              <stop offset="100%" stopColor="#7fa896" />
            </linearGradient>
          </defs>
          <circle
            cx="60"
            cy="60"
            r="54"
            fill="none"
            stroke="#59b892"
            strokeWidth="2.5"
            strokeDasharray="48 16 14 16"
            strokeLinecap="round"
            className="tire-loader-arc"
          />
          <circle cx="60" cy="60" r="44" fill="url(#gt-rubber)" stroke="#4c6b60" strokeWidth="3" />
          <circle
            cx="60"
            cy="60"
            r="44"
            fill="none"
            stroke="#8bb8a4"
            strokeWidth="4"
            strokeDasharray="6 9"
            opacity="0.45"
          />
          <circle cx="60" cy="60" r="24" fill="url(#gt-rim)" />
          <circle cx="60" cy="60" r="10" fill="#1b2b27" />
          {[0, 60, 120, 180, 240, 300].map((deg) => (
            <circle
              key={deg}
              cx={(60 + 16.5 * Math.cos((deg * Math.PI) / 180)).toFixed(4)}
              cy={(60 + 16.5 * Math.sin((deg * Math.PI) / 180)).toFixed(4)}
              r="2.4"
              fill="#1b2b27"
            />
          ))}
        </svg>
      </div>
      <div className="tire-loader-copy">
        <strong>{label}</strong>
        {!compact && <small>{sublabel}</small>}
        <div className="tire-loader-bar" aria-hidden="true">
          <span />
        </div>
      </div>
    </div>
  );
}
