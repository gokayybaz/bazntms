// k6 senaryosu — bazNTMS ingest yuk testi (Faz 4.5)
//
// Acik-dongu (open-loop) rate senaryosu: hedef ritim, agent sayisindan
// bagimsiz olarak korunur. Kurumsal kapasite hedefi (docs/enterprise-plan.html):
// 5000 agent @ 30 sn ≈ ≥170 ist/sn surekli.
//
// Kullanim:
//   k6 -e HUB=http://localhost:8080 -e ENROLL_TOKEN=xxx \
//      -e RATE=170 -e DURATION=10m loadtest/k6-ingest.js
//
// Gonderim ritmini k6 rate verir; agent kimlikleri setup()ta enroll edilir
// ve token havuzu VU'lar arasinda paylasilir.

import http from 'k6/http';
import { check, sleep } from 'k6';

const HUB = __ENV.HUB || 'http://localhost:8080';
const ENROLL_TOKEN = __ENV.ENROLL_TOKEN || '';
const RATE = Number(__ENV.RATE || 170);        // istek/sn
const DURATION = __ENV.DURATION || '5m';
const AGENTS = Number(__ENV.AGENTS || 200);    // token havuzu

export const options = {
  scenarios: {
    ingest: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 200,
      maxVUs: 2000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<250'],
    'http_req_duration{scenario:ingest}': ['p(95)<250'],
  },
};

export function setup() {
  if (!ENROLL_TOKEN) {
    throw new Error('ENROLL_TOKEN ortam degiskeni zorunlu');
  }
  const tokens = [];
  for (let i = 0; i < AGENTS; i++) {
    const res = http.post(
      `${HUB}/api/v1/agent/hello`,
      JSON.stringify({
        name: `k6-${i}`,
        site: 'k6',
        version: 'k6-1.0',
        protocol_version: 1,
      }),
      { headers: { 'X-Enroll-Token': ENROLL_TOKEN, 'Content-Type': 'application/json' } }
    );
    check(res, { 'enroll 200': (r) => r.status === 200 });
    tokens.push(res.json('agent_token'));
    sleep(0.02); // enroll bastirmasini yumusat
  }
  return { tokens };
}

export default function (data) {
  const token = data.tokens[Math.floor(Math.random() * data.tokens.length)];
  const now = Math.floor(Date.now() / 1000);
  const payload = JSON.stringify({
    ts: now,
    interfaces: [
      {
        name: 'eth0',
        rx_bytes: 1000000 + Math.floor(Math.random() * 500000),
        tx_bytes: 500000 + Math.floor(Math.random() * 250000),
        rx_packets: 1000 + Math.floor(Math.random() * 500),
        tx_packets: 800 + Math.floor(Math.random() * 400),
      },
    ],
    connections: [],
  });
  const res = http.post(`${HUB}/api/v1/agent/telemetry`, payload, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
  });
  check(res, { 'telemetry 200': (r) => r.status === 200 });
}
