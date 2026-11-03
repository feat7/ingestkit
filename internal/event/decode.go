package event

import (
	"bytes"
	"encoding/json"
	"io"
)

func decode(raw []byte, dst any, max int) error {
	if len(raw) > max {
		return &ValidationError{Message: "event or request exceeds byte limit", TooLarge: true}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return invalid("invalid event JSON or field")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return invalid("expected one JSON document")
	}
	return nil
}
func DecodeInput(raw []byte) (Input, error) {
	var in Input
	if err := decode(raw, &in, MaxEventBytes); err != nil {
		return in, err
	}
	return in, validate(in)
}
func DecodeBatch(raw []byte) ([]Input, error) {
	var req struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := decode(raw, &req, MaxRequestBytes); err != nil {
		return nil, err
	}
	if len(req.Events) == 0 || len(req.Events) > MaxBatchEvents {
		return nil, invalid("batch must contain 1–100 events")
	}
	out := make([]Input, len(req.Events))
	for i, b := range req.Events {
		in, err := DecodeInput(b)
		if err != nil {
			return nil, err
		}
		out[i] = in
	}
	return out, nil
}
func EncodeEnvelope(e Envelope) ([]byte, error) {
	if _, err := Accept(e.Input, e.TenantID, e.ReceivedAt); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	if b.Len() > MaxEnvelopeBytes {
		return nil, &ValidationError{Message: "broker envelope exceeds byte limit", TooLarge: true}
	}
	return b.Bytes(), nil
}
func DecodeEnvelope(raw []byte) (Envelope, error) {
	var e Envelope
	if err := decode(raw, &e, MaxEnvelopeBytes); err != nil {
		return e, err
	}
	return Accept(e.Input, e.TenantID, e.ReceivedAt)
}
