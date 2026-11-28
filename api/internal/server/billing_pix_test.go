package server

import (
	"strings"
	"testing"
)

func TestBuildPixPayloadContainsKeyAmountTxIDAndValidCRC(t *testing.T) {
	cfg := pixSettings()
	payload := buildPixPayload(cfg, 14900, "PIX-GT1234ABCD")
	if !strings.Contains(payload, "br.gov.bcb.pix") {
		t.Fatalf("missing PIX GUI in payload: %s", payload)
	}
	if !strings.Contains(payload, "getuliomoreira.engsoftware@gmail.com") {
		t.Fatalf("missing PIX key in payload: %s", payload)
	}
	if !strings.Contains(payload, "5406149.00") {
		t.Fatalf("missing formatted amount 149.00 in payload: %s", payload)
	}
	if !strings.Contains(payload, "PIXGT1234ABCD") {
		t.Fatalf("missing sanitized txid in payload: %s", payload)
	}
	if len(payload) < 10 {
		t.Fatalf("payload too short: %s", payload)
	}
	body := payload[:len(payload)-4]
	gotCRC := payload[len(payload)-4:]
	wantCRC := crc16CCITT(body)
	if gotCRC != wantCRC {
		t.Fatalf("invalid CRC16: got %s, want %s", gotCRC, wantCRC)
	}
}
