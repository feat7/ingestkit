# Delivery contract

The replacement API accepts explicit UUID event IDs, event types, RFC 3339 occurred-at timestamps and object payloads. Tenant and receipt time are assigned by the server. Retries retain IDs. HTTP 202 will mean broker acknowledgement, not database persistence. The old runtime remains in place until the replacement is verified.
