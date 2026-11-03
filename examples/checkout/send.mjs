// No npm dependencies. Reuse the same serialized IDs on every retry.
import { readFileSync } from 'node:fs';
const events = JSON.parse(readFileSync(new URL('./events.json', import.meta.url)));
const body = JSON.stringify(events);
const url = (process.env.API_URL || 'http://localhost:8080').replace(/\/$/, '') + '/v1/events/batch';
const key = process.env.INGESTKIT_API_KEY || 'local-demo-key';
let accepted = false;
for (let attempt = 0; attempt < 3; attempt++) {
  try {
    const response = await fetch(url, { method: 'POST', body,
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + key },
      signal: AbortSignal.timeout(8000) });
    if (response.status === 202) {
      const ack = await response.json();
      if (JSON.stringify(ack.event_ids) !== JSON.stringify(events.events.map(event => event.event_id)))
        throw new Error('acknowledgement IDs differ');
      console.log(JSON.stringify(ack)); accepted = true; break;
    }
    if (![429, 503].includes(response.status)) {
      console.error('HTTP ' + response.status); process.exit(1);
    }
  } catch { /* Delivery is ambiguous; retain the same IDs. */ }
  if (attempt < 2) await new Promise(resolve => setTimeout(resolve, 100 * (2 ** attempt)));
}
if (!accepted) { console.error('delivery uncertain; retry with the same event IDs'); process.exit(1); }
