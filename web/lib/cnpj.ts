// Mirrors the API checksum, including letters in the first twelve positions.
export function validCNPJ(value: string): boolean {
  const cnpj = value.toUpperCase().replace(/[./\s-]/g, "");
  if (!/^[A-Z0-9]{12}[0-9]{2}$/.test(cnpj) || /^(.)\1{13}$/.test(cnpj))
    return false;
  for (let n = 12; n < 14; n++) {
    let weight = n - 7;
    let sum = 0;
    for (let i = 0; i < n; i++) {
      sum += (cnpj.charCodeAt(i) - 48) * weight;
      if (--weight === 1) weight = 9;
    }
    const remainder = 11 - (sum % 11);
    if (Number(cnpj[n]) !== (remainder >= 10 ? 0 : remainder)) return false;
  }
  return true;
}
