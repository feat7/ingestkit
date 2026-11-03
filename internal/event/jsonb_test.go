package event

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestRejectsUnstoreableJSONAndText(t *testing.T) {
	bad := []string{`{"s":"\u0000"}`, `{"s":"\ud800"}`, `{"s":"\udc00"}`, `{"s":"\ud800\u0041"}`, `{"n":1e1000000}`, `{"n":1e-1000000}`, `{"n":1e131072}`, `{"n":1e-16384}`, `{"s":"` + string([]byte{255}) + `"}`, `{"n":` + strings.Repeat("[", 64) + "0" + strings.Repeat("]", 64) + "}"}
	for _, payload := range bad {
		t.Run(payload[:min(len(payload), 32)], func(t *testing.T) {
			raw := bytes.Replace(inputBytes(t, map[string]any{}), []byte(`"payload":{}`), []byte(`"payload":`+payload), 1)
			_, e := DecodeInput(raw)
			require.Error(t, e)
		})
	}
	for _, typ := range []string{"bad\x00type", string([]byte{255})} {
		in, _ := DecodeInput(inputBytes(t, map[string]any{}))
		in.EventType = typ
		_, e := Accept(in, "demo", time.Now())
		require.Error(t, e)
	}
	raw := bytes.Replace(inputBytes(t, map[string]any{}), []byte(`"order_completed"`), []byte(`"\ud800"`), 1)
	_, e := DecodeInput(raw)
	require.Error(t, e)
	in, _ := DecodeInput(inputBytes(t, map[string]any{}))
	_, e = Accept(in, "bad\x00tenant", time.Now())
	require.Error(t, e)
}
func TestAcceptsRepresentableJSONBoundaries(t *testing.T) {
	for _, payload := range []string{`{"s":"\ud83d\ude00"}`, `{"s":"\\u0000"}`, `{"s":"\/"}`, `{"n":1e131071}`, `{"n":1e-16383}`, `{"n":` + strings.Repeat("[", 63) + "0" + strings.Repeat("]", 63) + "}"} {
		in, _ := DecodeInput(inputBytes(t, map[string]any{}))
		in.Payload = json.RawMessage(payload)
		_, e := Accept(in, "demo", time.Now())
		require.NoError(t, e, payload[:min(len(payload), 40)])
	}
}
