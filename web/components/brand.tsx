import Link from "next/link";
export function Brand() {
  return (
    <Link href="/" className="brand" aria-label="GeevheitTire, início">
      <span className="brand-icon"><img src="/icon.svg" alt="" width={48} height={48}/></span>
      <span>
        Geevheit<span className="brand-light">Tire</span>
        <small>INTELIGÊNCIA EM CADA QUILÔMETRO</small>
      </span>
    </Link>
  );
}
