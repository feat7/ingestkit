// Copyable backend client. No npm dependencies and no redirect following.
import http from 'node:http';
import https from 'node:https';
import {readFileSync} from 'node:fs';
const uuid=/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i;
export class IngestError extends Error {
  constructor(message,{status=null,retryable=false,requestID=null}={}) {
    super(message);this.name='IngestError';this.status=status;this.retryable=retryable;
    this.requestID=typeof requestID==='string'&&uuid.test(requestID)?requestID:null;
  }
}
export class IngestClient {
  constructor({url,key,timeoutMs=8000,maxAttempts=3,caFile}={}) {
    try {
      const base=new URL(url);
      if(!['http:','https:'].includes(base.protocol)||base.username||base.password||base.search||base.hash||
         typeof key!=='string'||!/^[\x21-\x7e]+$/.test(key)||!Number.isFinite(timeoutMs)||timeoutMs<=0||
         !Number.isInteger(maxAttempts)||maxAttempts<1||maxAttempts>10)throw new Error();
      this.base=base;this.key=key;this.timeoutMs=timeoutMs;this.maxAttempts=maxAttempts;
      this.ca=caFile?readFileSync(caFile):undefined;
    }catch{throw new IngestError('invalid client configuration');}
  }
  send(event){return this._send([event],false);}
  sendBatch(events){return this._send(events,true);}
  async _send(events,batch) {
    let body,expected;
    try {
      expected=events.map(e=>e.event_id);
      if(!expected.length||expected.some(id=>typeof id!=='string'||!uuid.test(id)))throw new Error();
      expected=expected.map(id=>id.toLowerCase());
      body=JSON.stringify(batch?{events}:events[0]);
    }catch{throw new IngestError('event serialization or event ID is invalid');}
    const path=this.base.pathname.replace(/\/$/,'')+(batch?'/v1/events/batch':'/v1/events');
    let error;
    for(let attempt=0;attempt<this.maxAttempts;attempt++) {
      try{return await this._attempt(path,body,expected);}
      catch(e){error=e;if(!e.retryable)throw e;}
      if(attempt+1<this.maxAttempts)await new Promise(resolve=>setTimeout(resolve,100*2**attempt));
    }
    throw error;
  }
  _attempt(path,body,expected) {
    return new Promise((resolve,reject)=>{
      let done=false,timer;
      const finish=(error,value)=>{if(done)return;done=true;clearTimeout(timer);error?reject(error):resolve(value);};
      let req;
      try {
        const transport=this.base.protocol==='https:'?https:http;
        req=transport.request({protocol:this.base.protocol,hostname:this.base.hostname.replace(/^\[|\]$/g,''),port:this.base.port,
          path,method:'POST',ca:this.ca,rejectUnauthorized:true,headers:{'Content-Type':'application/json',
          'Content-Length':Buffer.byteLength(body),Authorization:'Bearer '+this.key}},res=>{
          const requestID=res.headers['x-request-id'],status=res.statusCode;
          if(status!==202){finish(new IngestError('HTTP '+status,{status,retryable:status===429||status>=500,requestID}));res.destroy();return;}
          const chunks=[];let length=0;
          res.on('data',chunk=>{length+=chunk.length;if(length>65536){finish(new IngestError('acknowledgement too large; delivery uncertain',{status,retryable:true,requestID}));res.destroy();}else chunks.push(chunk);});
          res.on('error',()=>finish(new IngestError('response interrupted; delivery uncertain',{retryable:true})));
          res.on('end',()=>{
            let ack;try{ack=JSON.parse(Buffer.concat(chunks).toString());}catch{}
            if(ack?.status!=='accepted'||JSON.stringify(ack?.event_ids)!==JSON.stringify(expected)){
              finish(new IngestError('acknowledgement differs; delivery uncertain',{status,retryable:true,requestID}));return;
            }
            finish(null,expected);
          });
        });
        req.on('error',e=>{
          const invalidTLS=['DEPTH_ZERO_SELF_SIGNED_CERT','SELF_SIGNED_CERT_IN_CHAIN','UNABLE_TO_VERIFY_LEAF_SIGNATURE','CERT_HAS_EXPIRED','ERR_TLS_CERT_ALTNAME_INVALID','UNABLE_TO_GET_ISSUER_CERT_LOCALLY'].includes(e.code);
          finish(new IngestError(invalidTLS?'TLS certificate is not trusted':'connection failed; delivery uncertain',{retryable:!invalidTLS}));
        });
        timer=setTimeout(()=>{finish(new IngestError('request timed out; delivery uncertain',{retryable:true}));req.destroy();},this.timeoutMs);
        req.end(body);
      }catch{req?.destroy();finish(new IngestError('request failed; delivery uncertain',{retryable:true}));}
    });
  }
}
