SELECT event_id,event_type,payload FROM ingestkit.events WHERE tenant_id='demo' ORDER BY occurred_at,event_id;
SELECT event_type,count(*) FROM ingestkit.events WHERE tenant_id='demo' GROUP BY event_type ORDER BY event_type;
