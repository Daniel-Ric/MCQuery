package ping

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadPacketRejectsOversizedPayloadBeforeAllocation(t *testing.T) {
	var packet bytes.Buffer
	writeVarInt(&packet, maxPacketLength+1)

	_, err := readPacket(&packet)
	if err == nil || !strings.Contains(err.Error(), "packet too large") {
		t.Fatalf("expected packet size error, got %v", err)
	}
}
