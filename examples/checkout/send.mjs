// No npm dependencies. Reuse serialized event IDs on every uncertain retry.
import { readFileSync } from 'node:fs';
const events = JSON.parse(readFileSync(process.env.EVENTS_FILE || new URL('./events.json', import.meta.url))).events;
const mode = process.env.SEND_MODE || 'batch';
if (!['single', 'batch'].includes(mode)) { console.error('SEND_MODE must be single or batch'); process.exit(1); }
const base = (process.env.API_URL || 'http://localhost:8080').replace(/\/$/, '');
const key = process.env.INGESTKIT_API_KEY || 'local-demo-key';
for (const group of mode === 'single' ? events.map(event => [event]) : [events]) {
  const body = JSON.stringify(mode === 'single' ? group[0] : { events: group });
  const url = base + (mode === 'single' ? '/v1/events' : '/v1/events/batch');
  let accepted = false;
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const response = await fetch(url, { method: 'POST', body,
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + key },
        signal: AbortSignal.timeout(8000) });
      if (response.status === 202) {
        const ack = await response.json();
        if (JSON.stringify(ack.event_ids) !== JSON.stringify(group.map(event => event.event_id)))
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
}
