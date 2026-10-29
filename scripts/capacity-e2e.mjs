// Read/write workload against an explicitly selected isolated API only.
// QA_CAPACITY_ACCOUNTS is created in a disposable Neon branch; never use production.
import fs from "node:fs";
import assert from "node:assert/strict";
import { performance } from "node:perf_hooks";
const base = process.env.CAPACITY_API_URL;
if (
  !base ||
  !/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(base) ||
  process.env.ALLOW_ISOLATED_LOAD !== "true"
)
  throw Error(
    "Explicit local isolated API and ALLOW_ISOLATED_LOAD=true are required",
  );
const fixtures = JSON.parse(
  fs.readFileSync(process.env.QA_CAPACITY_ACCOUNTS, "utf8"),
);
assert.equal(fixtures.clients.length, 100);
assert.equal(fixtures.clients.flatMap((c) => c.users).length, 500);
const routes = [
  "dashboard",
  "vehicles",
  "tires",
  "alerts",
  "entitlements",
  "billing",
  "map",
  "lifecycle",
  "reports",
  "inspection",
];
const durations = [],
  errors = [],
  statuses = {},
  identities = new Set();
let cursor = 0;
async function request(client, user, kind) {
  let path = kind,
    body;
  if (kind === "map") path = `vehicles/${client.vehicle_id}/tire-map`;
  if (kind === "lifecycle") path = `tires/${client.tire_id}/lifecycle`;
  if (kind === "reports") path = "tire-reports";
  if (kind === "reports") user = client.users[0];
  if (kind === "inspection") {
    path = `tires/${client.tire_id}/inspections`;
    body = {
      client_id: crypto.randomUUID(),
      tread_mm: 9,
      pressure_psi: 100,
      structural_damage: false,
    };
  }
  const start = performance.now();
  try {
    const res = await fetch(`${base}/api/v1/${path}`, {
      method: body ? "POST" : "GET",
      headers: {
        Authorization: `Bearer ${user.token}`,
        "Content-Type": "application/json",
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(25000),
    });
    const data = await res.json();
    if (res.ok) identities.add(user.id);
    statuses[res.status] = (statuses[res.status] || 0) + 1;
    const expected = body ? 201 : 200;
    if (res.status !== expected)
      errors.push({ route: kind, status: res.status, title: data.title });
    if (kind === "map" && res.ok)
      assert.equal(data.vehicle.id, client.vehicle_id);
  } catch (e) {
    errors.push({ route: kind, error: e.message });
  } finally {
    durations.push(performance.now() - start);
  }
}
const phases = [];
for (const concurrency of [10, 25, 50]) {
  const countBefore = durations.length,
    errorBefore = errors.length,
    start = performance.now();
  const phaseDurations = [];
  const workers = Array.from({ length: concurrency }, async () => {
    while (performance.now() - start < 15000) {
      const n = cursor++,
        c = fixtures.clients[n % 100],
        u = c.users[Math.floor(n / 100) % 5],
        kind = routes[(n + Math.floor(n / 100)) % routes.length],
        before = performance.now();
      await request(c, u, kind);
      phaseDurations.push(performance.now() - before);
    }
  });
  await Promise.all(workers);
  phaseDurations.sort((a, b) => a - b);
  const elapsed = (performance.now() - start) / 1000;
  const report = {
    concurrency,
    requests: durations.length - countBefore,
    errors: errors.length - errorBefore,
    elapsed_seconds: +elapsed.toFixed(2),
    requests_per_second: +((durations.length - countBefore) / elapsed).toFixed(
      2,
    ),
    p50_ms: +phaseDurations[Math.floor(phaseDurations.length * 0.5)].toFixed(1),
    p95_ms:
      +phaseDurations[Math.floor(phaseDurations.length * 0.95)].toFixed(1),
    p99_ms:
      +phaseDurations[Math.floor(phaseDurations.length * 0.99)].toFixed(1),
  };
  phases.push(report);
  console.log(JSON.stringify(report));
}
// Every identity must have been usable, and a foreign map must remain inaccessible.
const owner = fixtures.clients[0],
  foreign = fixtures.clients[1];
const denied = await fetch(
  `${base}/api/v1/vehicles/${foreign.vehicle_id}/tire-map`,
  { headers: { Authorization: `Bearer ${owner.users[0].token}` } },
);
assert.equal(denied.status, 404, "foreign vehicle map leaked");
const result = {
  scenario: {
    companies: 100,
    users: 500,
    vehicles: 1000,
    tires: 8000,
    base_inspections: 8000,
  },
  environment:
    "Local Go API with isolated real Neon PostgreSQL; not a Render capacity guarantee",
  phases,
  statuses,
  errors,
  total_requests: durations.length,
  distinct_identities: identities.size,
};
fs.writeFileSync(
  process.env.CAPACITY_REPORT || "/private/tmp/geevheit-capacity-report.json",
  JSON.stringify(result, null, 2),
);
assert.equal(errors.length, 0, JSON.stringify(errors.slice(0, 5)));
console.log(
  "PASS workload, concurrent writes and foreign-tenant denial; report saved.",
);
