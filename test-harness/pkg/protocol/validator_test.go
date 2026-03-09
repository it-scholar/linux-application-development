package protocol

import (
	"bytes"
	"testing"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator(nil)
	if validator == nil {
		t.Fatal("NewValidator returned nil")
	}

	if validator.spec == nil {
		t.Error("validator should have default spec")
	}
}

func TestDefaultSpec(t *testing.T) {
	spec := DefaultSpec()

	if spec.Version != CurrentVersion {
		t.Errorf("expected version %d, got %d", CurrentVersion, spec.Version)
	}

	if spec.MagicNumber != DefaultMagic {
		t.Errorf("expected magic 0x%08x, got 0x%08x", DefaultMagic, spec.MagicNumber)
	}

	if spec.MaxPayload != MaxPayloadSize {
		t.Errorf("expected max payload %d, got %d", MaxPayloadSize, spec.MaxPayload)
	}

	// Should have message types defined
	if len(spec.MessageTypes) == 0 {
		t.Error("spec should have message types defined")
	}

	// Check heartbeat type exists
	heartbeat, ok := spec.MessageTypes[MsgHeartbeat]
	if !ok {
		t.Error("heartbeat message type not defined")
	} else {
		if heartbeat.Name != "Heartbeat" {
			t.Errorf("expected name 'Heartbeat', got '%s'", heartbeat.Name)
		}
		if heartbeat.HasPayload {
			t.Error("heartbeat should not have payload")
		}
	}
}

func TestValidateHeader(t *testing.T) {
	validator := NewValidator(nil)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid header",
			data:    createValidHeader(t),
			wantErr: false,
		},
		{
			name:    "too short",
			data:    []byte{0x00, 0x01, 0x02},
			wantErr: true,
		},
		{
			name:    "wrong magic",
			data:    createHeaderWithWrongMagic(),
			wantErr: true,
		},
		{
			name:    "wrong version",
			data:    createHeaderWithWrongVersion(),
			wantErr: true,
		},
		{
			name:    "unknown message type",
			data:    createHeaderWithUnknownType(),
			wantErr: true,
		},
		{
			name:    "payload too large",
			data:    createHeaderWithHugePayload(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, err := validator.ValidateHeader(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if header == nil {
					t.Error("expected header, got nil")
				}
			}
		})
	}
}

func TestValidateMessage(t *testing.T) {
	validator := NewValidator(nil)

	// Valid message with payload
	msg := createValidMessage(t, MsgDataRequest, []byte("test payload"))

	header, payload, err := validator.ValidateMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if header == nil {
		t.Error("expected header, got nil")
	}

	if header.Type != MsgDataRequest {
		t.Errorf("expected type %d, got %d", MsgDataRequest, header.Type)
	}

	if !bytes.Equal(payload, []byte("test payload")) {
		t.Errorf("expected payload 'test payload', got '%s'", string(payload))
	}
}

func TestValidateMessageNoPayload(t *testing.T) {
	validator := NewValidator(nil)

	// Heartbeat should have no payload
	msg := createValidMessage(t, MsgHeartbeat, []byte{})

	_, _, err := validator.ValidateMessage(msg)
	if err != nil {
		t.Errorf("heartbeat without payload should be valid: %v", err)
	}

	// Heartbeat with payload should fail
	msgWithPayload := createValidMessage(t, MsgHeartbeat, []byte("payload"))
	_, _, err = validator.ValidateMessage(msgWithPayload)
	if err == nil {
		t.Error("heartbeat with payload should fail")
	}
}

func TestCreateHeader(t *testing.T) {
	validator := NewValidator(nil)

	header := validator.CreateHeader(MsgHeartbeat, 0, 123)

	if header.Magic != DefaultMagic {
		t.Errorf("expected magic 0x%08x, got 0x%08x", DefaultMagic, header.Magic)
	}

	if header.Version != CurrentVersion {
		t.Errorf("expected version %d, got %d", CurrentVersion, header.Version)
	}

	if header.Type != MsgHeartbeat {
		t.Errorf("expected type %d, got %d", MsgHeartbeat, header.Type)
	}

	if header.Sequence != 123 {
		t.Errorf("expected sequence 123, got %d", header.Sequence)
	}
}

func TestEncodeHeader(t *testing.T) {
	validator := NewValidator(nil)

	header := validator.CreateHeader(MsgHeartbeat, 0, 1)
	encoded := validator.EncodeHeader(header)

	if len(encoded) != HeaderSize {
		t.Errorf("expected encoded header size %d, got %d", HeaderSize, len(encoded))
	}

	// Verify we can decode it back
	decoded, err := validator.ValidateHeader(encoded)
	if err != nil {
		t.Fatalf("failed to decode header: %v", err)
	}

	if decoded.Magic != header.Magic {
		t.Error("magic mismatch after encode/decode")
	}
	if decoded.Version != header.Version {
		t.Error("version mismatch after encode/decode")
	}
	if decoded.Type != header.Type {
		t.Error("type mismatch after encode/decode")
	}
}

func TestCreateMessage(t *testing.T) {
	validator := NewValidator(nil)

	payload := []byte("hello world")
	msg := validator.CreateMessage(MsgDataRequest, payload, 1)

	// Should be header + payload
	expectedLen := HeaderSize + len(payload)
	if len(msg) != expectedLen {
		t.Errorf("expected message length %d, got %d", expectedLen, len(msg))
	}

	// Validate the message
	header, decodedPayload, err := validator.ValidateMessage(msg)
	if err != nil {
		t.Fatalf("failed to validate created message: %v", err)
	}

	if header.Type != MsgDataRequest {
		t.Error("message type mismatch")
	}

	if !bytes.Equal(decodedPayload, payload) {
		t.Error("payload mismatch")
	}
}

func TestComplianceReport(t *testing.T) {
	validator := NewValidator(nil)

	results := []ValidationResult{
		{Valid: true, Message: 0},
		{Valid: true, Message: 1},
		{Valid: false, Message: 2, Error: "validation failed"},
	}

	report := validator.ComplianceReport(results)

	if report.TotalMessages != 3 {
		t.Errorf("expected 3 total messages, got %d", report.TotalMessages)
	}

	if report.ValidMessages != 2 {
		t.Errorf("expected 2 valid messages, got %d", report.ValidMessages)
	}

	if report.InvalidMessages != 1 {
		t.Errorf("expected 1 invalid message, got %d", report.InvalidMessages)
	}

	if report.Compliant {
		t.Error("report should not be compliant with invalid messages")
	}

	if len(report.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(report.Errors))
	}
}

// Helper functions

func createValidHeader(t *testing.T) []byte {
	validator := NewValidator(nil)
	header := validator.CreateHeader(MsgHeartbeat, 0, 1)
	return validator.EncodeHeader(header)
}

func createHeaderWithWrongMagic() []byte {
	validator := NewValidator(nil)
	header := validator.CreateHeader(MsgHeartbeat, 0, 1)
	header.Magic = 0xDEADBEEF
	return validator.EncodeHeader(header)
}

func createHeaderWithWrongVersion() []byte {
	validator := NewValidator(nil)
	header := validator.CreateHeader(MsgHeartbeat, 0, 1)
	header.Version = 999
	return validator.EncodeHeader(header)
}

func createHeaderWithUnknownType() []byte {
	validator := NewValidator(nil)
	header := validator.CreateHeader(MsgHeartbeat, 0, 1)
	header.Type = 0xFF // Unknown type
	return validator.EncodeHeader(header)
}

func createHeaderWithHugePayload() []byte {
	validator := NewValidator(nil)
	header := validator.CreateHeader(MsgDataRequest, MaxPayloadSize+1000, 1)
	return validator.EncodeHeader(header)
}

func createValidMessage(t *testing.T, msgType uint8, payload []byte) []byte {
	validator := NewValidator(nil)
	header := validator.CreateHeader(msgType, uint32(len(payload)), 1)

	msg := validator.EncodeHeader(header)
	msg = append(msg, payload...)
	return msg
}
