import { readFileSync, writeFileSync } from 'node:fs';

const [csvPath, chaosPath, outPath] = process.argv.slice(2);
const BUCKET = 5;

const rows = readFileSync(csvPath, 'utf8').split('\n');
const reqs = [];
for (const line of rows) {
  if (!line.startsWith('http_reqs,')) continue;
  const f = line.split(',');
  reqs.push({ ts: Number(f[1]), ok: f[13] === '201' });
}
const t0 = Math.min(...reqs.map(r => r.ts));
const t1 = Math.max(...reqs.map(r => r.ts));
const nb = Math.floor((t1 - t0 + 1) / BUCKET);
const okB = Array(nb).fill(0);
const failB = Array(nb).fill(0);
for (const r of reqs) {
  const i = Math.floor((r.ts - t0) / BUCKET);
  if (i >= nb) continue;
  (r.ok ? okB : failB)[i]++;
}
const okRps = okB.map(v => v / BUCKET);
const failRps = failB.map(v => v / BUCKET);

const kills = readFileSync(chaosPath, 'utf8')
  .split('\n')
  .filter(Boolean)
  .map(l => Number(l.split(' ')[0]))
  .filter(ts => ts >= t0 && ts <= t1);

const W = 800, H = 400, ML = 60, MR = 20, MT = 40, MB = 50;
const PW = W - ML - MR, PH = H - MT - MB;
const yMax = 40;
const x = ts => ML + ((ts - t0) / (t1 - t0)) * PW;
const xi = i => ML + ((i * BUCKET + BUCKET / 2) / (t1 - t0)) * PW;
const y = v => MT + PH - (v / yMax) * PH;

const path = arr =>
  arr.map((v, i) => `${i ? 'L' : 'M'}${xi(i).toFixed(1)},${y(v).toFixed(1)}`).join(' ');

const yTicks = [0, 10, 20, 30, 40];
const xTicks = [];
for (let s = 0; s <= t1 - t0; s += 30) xTicks.push(s);

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" font-family="DejaVu Sans, Verdana, sans-serif">
  <rect width="${W}" height="${H}" fill="#ffffff"/>
  <text x="${ML + PW / 2}" y="22" font-size="13" fill="#000" text-anchor="middle">rps во время хаос-теста (под убивается каждые 5 сек)</text>
  ${kills.map(ts => `<line x1="${x(ts).toFixed(1)}" y1="${MT}" x2="${x(ts).toFixed(1)}" y2="${MT + PH}" stroke="#bbbbbb" stroke-width="1" stroke-dasharray="4 3"/>`).join('\n  ')}
  <rect x="${ML}" y="${MT}" width="${PW}" height="${PH}" fill="none" stroke="#000" stroke-width="0.8"/>
  ${yTicks.map(v => `<line x1="${ML - 4}" y1="${y(v)}" x2="${ML}" y2="${y(v)}" stroke="#000" stroke-width="0.8"/>
  <text x="${ML - 8}" y="${y(v) + 3.5}" font-size="10" fill="#000" text-anchor="end">${v}</text>`).join('\n  ')}
  ${xTicks.map(s => `<line x1="${ML + (s / (t1 - t0)) * PW}" y1="${MT + PH}" x2="${ML + (s / (t1 - t0)) * PW}" y2="${MT + PH + 4}" stroke="#000" stroke-width="0.8"/>
  <text x="${ML + (s / (t1 - t0)) * PW}" y="${MT + PH + 16}" font-size="10" fill="#000" text-anchor="middle">${s}</text>`).join('\n  ')}
  <text x="${ML + PW / 2}" y="${H - 12}" font-size="11" fill="#000" text-anchor="middle">секунды от старта k6</text>
  <text x="18" y="${MT + PH / 2}" font-size="11" fill="#000" text-anchor="middle" transform="rotate(-90 18 ${MT + PH / 2})">req/s</text>
  <path d="${path(okRps)}" fill="none" stroke="#1f77b4" stroke-width="1.5"/>
  <path d="${path(failRps)}" fill="none" stroke="#d62728" stroke-width="1.5"/>
  <g font-size="10">
    <rect x="${ML + PW - 150}" y="${MT + 8}" width="140" height="52" fill="#ffffff" stroke="#cccccc" stroke-width="0.8"/>
    <line x1="${ML + PW - 142}" y1="${MT + 20}" x2="${ML + PW - 122}" y2="${MT + 20}" stroke="#1f77b4" stroke-width="1.5"/>
    <text x="${ML + PW - 116}" y="${MT + 23}" fill="#000">успешные (201)</text>
    <line x1="${ML + PW - 142}" y1="${MT + 36}" x2="${ML + PW - 122}" y2="${MT + 36}" stroke="#d62728" stroke-width="1.5"/>
    <text x="${ML + PW - 116}" y="${MT + 39}" fill="#000">ошибки</text>
    <line x1="${ML + PW - 142}" y1="${MT + 52}" x2="${ML + PW - 122}" y2="${MT + 52}" stroke="#bbbbbb" stroke-width="1" stroke-dasharray="4 3"/>
    <text x="${ML + PW - 116}" y="${MT + 55}" fill="#000">kill пода</text>
  </g>
</svg>
`;
writeFileSync(outPath, svg);
console.log(`ok=${reqs.filter(r => r.ok).length} fail=${reqs.filter(r => !r.ok).length} kills=${kills.length} -> ${outPath}`);
