import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import https from 'node:https';
import {mkdtempSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {IngestClient,IngestError} from './ingest-client.mjs';

const event={event_id:'dc12060f-f3a6-4ed3-9ea9-2d396b260b41',event_type:'order_completed',occurred_at:'2026-11-04T00:00:00Z',payload:{private:'payload-canary'}};
const requestID='f8c22d85-5e93-4cb5-a456-18b0f104bcca';
async function withServer(actions,run,tls){
 const seen=[];
 const handler=async(req,res)=>{
  const chunks=[];for await(const chunk of req)chunks.push(chunk);
  seen.push({path:req.url,body:Buffer.concat(chunks).toString(),key:req.headers.authorization});
  const action=actions[Math.min(seen.length-1,actions.length-1)];
  if(action==='close'){req.socket.destroy();return;}
  if(action==='hang'){setTimeout(()=>res.destroy(),250);return;}
  res.writeHead(action[0],{'X-Request-ID':requestID});res.end(JSON.stringify({status:'accepted',event_ids:action[1],message:'secret-key payload-canary postgres://credential'}));
 };
 const server=tls?https.createServer(tls,handler):http.createServer(handler);
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 try{await run(`${tls?'https':'http'}://127.0.0.1:${server.address().port}`,seen);}
 finally{server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
}
test('import has no I/O side effects',()=>{
 assert.equal(execFileSync(process.execPath,['--input-type=module','-e',"await import('./examples/checkout/send.mjs'); await import('./examples/checkout/ingest-client.mjs');"],{env:{...process.env,EVENTS_FILE:'/absent'}}).toString(),'');
});
test('single and batch exact ACK',async()=>{
 for(const batch of [false,true])await withServer([[202,[event.event_id]]],async(url,seen)=>{
  const client=new IngestClient({url,key:'secret-key'});
  assert.deepEqual(await(batch?client.sendBatch([event]):client.send(event)),[event.event_id]);
  assert.equal(seen[0].path,batch?'/v1/events/batch':'/v1/events');assert.equal(seen[0].key,'Bearer secret-key');
 });
});
test('transient retries reuse bytes, and stop',async()=>{
 for(const failure of [429,503,'close','hang']){
  const action=typeof failure==='string'?failure:[failure,[]];
  await withServer([action,[202,[event.event_id]]],async(url,seen)=>{
   assert.deepEqual(await new IngestClient({url,key:'secret-key',timeoutMs:60}).send(event),[event.event_id]);
   assert.equal(seen.length,2);assert.equal(seen[0].body,seen[1].body);
  });
  await withServer([action],async(url,seen)=>{
   const at=Date.now();await assert.rejects(new IngestClient({url,key:'secret-key',timeoutMs:60,maxAttempts:2}).send(event),e=>{
    assert(e instanceof IngestError);assert.equal(e.retryable,true);
    for(const secret of ['secret-key','payload-canary','postgres://'])assert(!String(e).includes(secret));return true;
   });assert.equal(seen.length,2);assert(Date.now()-at<2000);
  });
 }
});
test('permanent rejection does not retry; safe request ID',async()=>{
 for(const status of [400,401,413])await withServer([[status,[]]],async(url,seen)=>{
  await assert.rejects(new IngestClient({url,key:'secret-key'}).send(event),e=>{assert.equal(e.status,status);assert.equal(e.retryable,false);assert.equal(e.requestID,requestID);return true;});assert.equal(seen.length,1);
 });
});
test('mismatched ACK and redirect cannot succeed',async()=>{
 for(const action of [[202,['wrong']],[302,[]]])await withServer([action],async(url,seen)=>{
  await assert.rejects(new IngestClient({url,key:'secret-key',maxAttempts:2}).send(event),IngestError);assert(seen.length<=2);
 });
});
test('TLS requires explicit CA',async()=>{
 const dir=mkdtempSync(join(tmpdir(),'ingestkit-client-'));
 try{
  const config=join(dir,'openssl.conf'),cert=join(dir,'cert.pem'),key=join(dir,'key.pem');
  writeFileSync(config,'[req]\nprompt=no\ndistinguished_name=dn\nx509_extensions=v3\n[dn]\nCN=localhost\n[v3]\nsubjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=critical,CA:TRUE\n');
  execFileSync('openssl',['req','-x509','-nodes','-newkey','rsa:2048','-days','1','-config',config,'-out',cert,'-keyout',key],{stdio:'ignore'});
  await withServer([[202,[event.event_id]]],async(url)=>{
   await assert.rejects(new IngestClient({url,key:'secret-key',maxAttempts:1}).send(event),IngestError);
   assert.deepEqual(await new IngestClient({url,key:'secret-key',caFile:cert}).send(event),[event.event_id]);
  },{cert:readFileSync(cert),key:readFileSync(key)});
 }finally{rmSync(dir,{recursive:true,force:true});}
});

test('invalid credential header is a permanent safe configuration error',()=>{
 assert.throws(()=>new IngestClient({url:'http://localhost',key:'private-key\nInjected: payload-canary'}),e=>e instanceof IngestError&&!e.retryable&&!String(e).includes('private-key'));
});
