package event

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func inputBytes(t *testing.T, payload any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(map[string]any{"event_id": "00000000-0000-4000-8000-000000000001", "event_type": "order_completed", "occurred_at": "2026-11-03T00:00:00Z", "payload": payload})
	require.NoError(t, err)
	return bytes.TrimSpace(buf.Bytes())
}
func TestDecodeValidEvent(t *testing.T) {
	v, err := DecodeInput(inputBytes(t, map[string]any{"amount": 12}))
	require.NoError(t, err)
	require.Equal(t, "order_completed", v.EventType)
}
func TestDecodeRejectsInvalidEnvelope(t *testing.T) {
	for _, mut := range []func(map[string]any){
		func(m map[string]any) { delete(m, "event_id") }, func(m map[string]any) { m["event_id"] = uuid.Nil.String() },
		func(m map[string]any) { m["event_id"] = "bad" }, func(m map[string]any) { m["event_type"] = "" },
		func(m map[string]any) { m["event_type"] = strings.Repeat("x", 129) }, func(m map[string]any) { m["occurred_at"] = "no" },
		func(m map[string]any) { m["payload"] = nil }, func(m map[string]any) { m["payload"] = []int{1} },
		func(m map[string]any) { m["payload"] = 12 }, func(m map[string]any) { m["tenant_id"] = "evil" },
		func(m map[string]any) { m["received_at"] = "2026-11-03T00:00:00Z" },
	} {
		var m map[string]any
		require.NoError(t, json.Unmarshal(inputBytes(t, map[string]any{}), &m))
		mut(m)
		b, _ := json.Marshal(m)
		_, err := DecodeInput(b)
		require.Error(t, err, string(b))
	}
	_, err := DecodeInput(append(inputBytes(t, map[string]any{}), []byte(" {}")...))
	require.Error(t, err)
}
func TestEventByteLimits(t *testing.T) {
	_, err := DecodeInput(inputBytes(t, map[string]any{"text": strings.Repeat("界", MaxEventBytes/2)}))
	require.Error(t, err)
	require.True(t, IsTooLarge(err))
}
func TestBatchLimitsAndValidation(t *testing.T) {
	raw := inputBytes(t, map[string]any{})
	items := bytes.Repeat(append(raw, ','), 100)
	items = items[:len(items)-1]
	batch := append([]byte("{\"events\":["), items...)
	batch = append(batch, ']', '}')
	got, err := DecodeBatch(batch)
	require.NoError(t, err)
	require.Len(t, got, 100)
	_, err = DecodeBatch(bytes.Replace(batch, []byte("]}"), append(append([]byte(","), raw...), ']', '}'), 1))
	require.Error(t, err)
	_, err = DecodeBatch([]byte("{\"events\":[]}"))
	require.Error(t, err)
	_, err = DecodeBatch([]byte("{\"events\":[{}]}"))
	require.Error(t, err)
	large := inputBytes(t, map[string]any{"x": strings.Repeat("x", MaxEventBytes)})
	_, err = DecodeBatch(append(append([]byte("{\"events\":["), large...), ']', '}'))
	require.True(t, IsTooLarge(err))
	_, err = DecodeBatch(bytes.Repeat([]byte(" "), MaxRequestBytes+1))
	require.True(t, IsTooLarge(err))
}
func TestAcceptUsesServerIdentity(t *testing.T) {
	in, err := DecodeInput(inputBytes(t, map[string]any{}))
	require.NoError(t, err)
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	env, err := Accept(in, "tenant-a", now)
	require.NoError(t, err)
	require.Equal(t, "tenant-a", env.TenantID)
	require.Equal(t, now, env.ReceivedAt)
	_, err = Accept(in, "", now)
	require.Error(t, err)
}
func TestDecodeBrokerEnvelope(t *testing.T) {
	in, _ := DecodeInput(inputBytes(t, map[string]any{}))
	env, _ := Accept(in, "demo", time.Now().UTC())
	b, err := EncodeEnvelope(env)
	require.NoError(t, err)
	got, err := DecodeEnvelope(b)
	require.NoError(t, err)
	require.Equal(t, env.EventID, got.EventID)
	env.TenantID = ""
	b, _ = json.Marshal(env)
	_, err = DecodeEnvelope(b)
	require.Error(t, err)
}
func TestEnvelopeEncodingAtInputLimit(t *testing.T) {
	raw := inputBytes(t, map[string]any{"text": strings.Repeat("<>&", 30000)})
	raw = append(raw, bytes.Repeat([]byte(" "), MaxEventBytes-len(raw))...)
	in, err := DecodeInput(raw)
	require.NoError(t, err)
	env, err := Accept(in, "demo", time.Now().UTC())
	require.NoError(t, err)
	b, err := EncodeEnvelope(env)
	require.NoError(t, err)
	require.LessOrEqual(t, len(b), MaxEnvelopeBytes)
	_, err = DecodeEnvelope(b)
	require.NoError(t, err)
}
