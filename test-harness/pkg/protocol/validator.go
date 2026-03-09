// Package protocol provides binary protocol validation and testing
package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Validator validates binary protocol compliance
type Validator struct {
	spec *Spec
}

// Spec represents the protocol specification
type Spec struct {
	Version      uint16
	MagicNumber  uint32
	MaxPayload   uint32
	MessageTypes map[uint8]MessageType
}

// MessageType represents a protocol message type
type MessageType struct {
	ID             uint8
	Name           string
	HasPayload     bool
	MaxPayload     uint32
	RequiredFields []string
}

// Header represents the protocol header
type Header struct {
	Magic     uint32
	Version   uint16
	Type      uint8
	Flags     uint8
	Length    uint32
	Sequence  uint32
	Timestamp uint64
}

const (
	// Protocol constants
	DefaultMagic   = 0x57454154 // "WEAT" in ASCII
	CurrentVersion = 1
	HeaderSize     = 24
	MaxPayloadSize = 65536

	// Message types
	MsgHeartbeat    = 0x01
	MsgDataRequest  = 0x02
	MsgDataResponse = 0x03
	MsgError        = 0x04
	MsgQuery        = 0x05
	MsgQueryResult  = 0x06
)

// NewValidator creates a new protocol validator
func NewValidator(spec *Spec) *Validator {
	if spec == nil {
		spec = DefaultSpec()
	}
	return &Validator{spec: spec}
}

// DefaultSpec returns the default protocol specification
func DefaultSpec() *Spec {
	return &Spec{
		Version:     CurrentVersion,
		MagicNumber: DefaultMagic,
		MaxPayload:  MaxPayloadSize,
		MessageTypes: map[uint8]MessageType{
			MsgHeartbeat: {
				ID:         MsgHeartbeat,
				Name:       "Heartbeat",
				HasPayload: false,
				MaxPayload: 0,
			},
			MsgDataRequest: {
				ID:             MsgDataRequest,
				Name:           "DataRequest",
				HasPayload:     true,
				MaxPayload:     1024,
				RequiredFields: []string{"station_id", "start_date", "end_date"},
			},
			MsgDataResponse: {
				ID:         MsgDataResponse,
				Name:       "DataResponse",
				HasPayload: true,
				MaxPayload: MaxPayloadSize,
			},
			MsgError: {
				ID:             MsgError,
				Name:           "Error",
				HasPayload:     true,
				MaxPayload:     256,
				RequiredFields: []string{"error_code", "error_message"},
			},
			MsgQuery: {
				ID:             MsgQuery,
				Name:           "Query",
				HasPayload:     true,
				MaxPayload:     512,
				RequiredFields: []string{"query_type", "query_params"},
			},
			MsgQueryResult: {
				ID:         MsgQueryResult,
				Name:       "QueryResult",
				HasPayload: true,
				MaxPayload: MaxPayloadSize,
			},
		},
	}
}

// ValidateHeader validates a protocol header
func (v *Validator) ValidateHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("header too short: got %d bytes, need %d", len(data), HeaderSize)
	}

	header := &Header{}
	reader := bytes.NewReader(data)

	// Parse header fields
	binary.Read(reader, binary.BigEndian, &header.Magic)
	binary.Read(reader, binary.BigEndian, &header.Version)
	binary.Read(reader, binary.BigEndian, &header.Type)
	binary.Read(reader, binary.BigEndian, &header.Flags)
	binary.Read(reader, binary.BigEndian, &header.Length)
	binary.Read(reader, binary.BigEndian, &header.Sequence)
	binary.Read(reader, binary.BigEndian, &header.Timestamp)

	// Validate magic number
	if header.Magic != v.spec.MagicNumber {
		return nil, fmt.Errorf("invalid magic number: got 0x%08x, want 0x%08x",
			header.Magic, v.spec.MagicNumber)
	}

	// Validate version
	if header.Version != v.spec.Version {
		return nil, fmt.Errorf("unsupported version: got %d, want %d",
			header.Version, v.spec.Version)
	}

	// Validate message type
	if _, ok := v.spec.MessageTypes[header.Type]; !ok {
		return nil, fmt.Errorf("unknown message type: 0x%02x", header.Type)
	}

	// Validate payload length
	if header.Length > v.spec.MaxPayload {
		return nil, fmt.Errorf("payload too large: got %d, max %d",
			header.Length, v.spec.MaxPayload)
	}

	return header, nil
}

// ValidateMessage validates a complete message (header + payload)
func (v *Validator) ValidateMessage(data []byte) (*Header, []byte, error) {
	if len(data) < HeaderSize {
		return nil, nil, fmt.Errorf("message too short: got %d bytes", len(data))
	}

	header, err := v.ValidateHeader(data[:HeaderSize])
	if err != nil {
		return nil, nil, err
	}

	// Check message type constraints
	msgType := v.spec.MessageTypes[header.Type]

	if !msgType.HasPayload && header.Length > 0 {
		return nil, nil, fmt.Errorf("message type 0x%02x should not have payload", header.Type)
	}

	if msgType.HasPayload && header.Length == 0 {
		return nil, nil, fmt.Errorf("message type 0x%02x requires payload", header.Type)
	}

	// Check payload bounds
	if header.Length > msgType.MaxPayload {
		return nil, nil, fmt.Errorf("payload exceeds max for type 0x%02x: got %d, max %d",
			header.Type, header.Length, msgType.MaxPayload)
	}

	// Extract payload
	expectedLen := HeaderSize + int(header.Length)
	if len(data) < expectedLen {
		return nil, nil, fmt.Errorf("truncated message: got %d bytes, need %d",
			len(data), expectedLen)
	}

	payload := data[HeaderSize:expectedLen]

	return header, payload, nil
}

// CreateHeader creates a protocol header
func (v *Validator) CreateHeader(msgType uint8, payloadLen uint32, sequence uint32) *Header {
	return &Header{
		Magic:     v.spec.MagicNumber,
		Version:   v.spec.Version,
		Type:      msgType,
		Flags:     0,
		Length:    payloadLen,
		Sequence:  sequence,
		Timestamp: uint64(0), // Should be set to actual timestamp
	}
}

// EncodeHeader encodes a header to bytes
func (v *Validator) EncodeHeader(header *Header) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, header.Magic)
	binary.Write(buf, binary.BigEndian, header.Version)
	binary.Write(buf, binary.BigEndian, header.Type)
	binary.Write(buf, binary.BigEndian, header.Flags)
	binary.Write(buf, binary.BigEndian, header.Length)
	binary.Write(buf, binary.BigEndian, header.Sequence)
	binary.Write(buf, binary.BigEndian, header.Timestamp)
	return buf.Bytes()
}

// CreateMessage creates a complete message (header + payload)
func (v *Validator) CreateMessage(msgType uint8, payload []byte, sequence uint32) []byte {
	header := v.CreateHeader(msgType, uint32(len(payload)), sequence)
	header.Timestamp = uint64(0) // Set to current time in nanoseconds

	msg := v.EncodeHeader(header)
	msg = append(msg, payload...)
	return msg
}

// ValidateStream validates a stream of messages
func (v *Validator) ValidateStream(reader io.Reader) ([]ValidationResult, error) {
	results := make([]ValidationResult, 0)
	sequence := uint32(0)

	for {
		// Read header
		headerBuf := make([]byte, HeaderSize)
		_, err := io.ReadFull(reader, headerBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			results = append(results, ValidationResult{
				Valid:   false,
				Error:   fmt.Sprintf("failed to read header: %v", err),
				Message: sequence,
			})
			continue
		}

		header, err := v.ValidateHeader(headerBuf)
		if err != nil {
			results = append(results, ValidationResult{
				Valid:   false,
				Error:   err.Error(),
				Message: sequence,
			})
			continue
		}

		// Read payload if present
		var payload []byte
		if header.Length > 0 {
			payload = make([]byte, header.Length)
			_, err = io.ReadFull(reader, payload)
			if err != nil {
				results = append(results, ValidationResult{
					Valid:   false,
					Error:   fmt.Sprintf("failed to read payload: %v", err),
					Message: sequence,
				})
				continue
			}
		}

		results = append(results, ValidationResult{
			Valid:   true,
			Header:  header,
			Payload: payload,
			Message: sequence,
		})

		sequence++
	}

	return results, nil
}

// ValidationResult represents a single message validation result
type ValidationResult struct {
	Valid   bool
	Header  *Header
	Payload []byte
	Error   string
	Message uint32
}

// ComplianceReport generates a compliance report
func (v *Validator) ComplianceReport(results []ValidationResult) *Report {
	report := &Report{
		TotalMessages:   len(results),
		ValidMessages:   0,
		InvalidMessages: 0,
		Errors:          make([]string, 0),
		MessageTypes:    make(map[uint8]int),
	}

	for _, result := range results {
		if result.Valid {
			report.ValidMessages++
			if result.Header != nil {
				report.MessageTypes[result.Header.Type]++
			}
		} else {
			report.InvalidMessages++
			report.Errors = append(report.Errors,
				fmt.Sprintf("Message %d: %s", result.Message, result.Error))
		}
	}

	report.Compliant = report.InvalidMessages == 0
	return report
}

// Report represents a protocol compliance report
type Report struct {
	Compliant       bool
	TotalMessages   int
	ValidMessages   int
	InvalidMessages int
	Errors          []string
	MessageTypes    map[uint8]int
}
