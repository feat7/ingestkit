import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {IngestClient,IngestError} from './ingest-client.mjs';
export async function main() {
  try {
    const events=JSON.parse(readFileSync(process.env.EVENTS_FILE||new URL('./events.json',import.meta.url))).events;
    const mode=process.env.SEND_MODE||'batch';
    if(!['single','batch'].includes(mode))throw new Error();
    const client=new IngestClient({url:process.env.API_URL||'http://localhost:8080',key:process.env.INGESTKIT_API_KEY||'local-demo-key',caFile:process.env.INGESTKIT_CA_FILE});
    for(const group of mode==='single'?events.map(e=>[e]):[events]) {
      const ids=await(mode==='single'?client.send(group[0]):client.sendBatch(group));
      console.log(JSON.stringify({status:'accepted',event_ids:ids}));
    }
    return 0;
  }catch(e){console.error(e instanceof IngestError?e.message:'invalid fixture or caller configuration');return 1;}
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))process.exitCode=await main();
