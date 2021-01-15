package status

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hieutran-individual/routing/codes"
	"github.com/hieutran-individual/routing/pb"
)

// Status represents an RPC status code, message, and details.  It is immutable
// and should be created with New, Newf, or FromProto.
type Status struct {
	*pb.Status
}

// New returns a Status representing c and msg.
func New(c codes.Code, msg string) *Status {
	return &Status{&pb.Status{
		Code: int32(c), Message: msg,
	}}
}

func Newf(c codes.Code, format string, p ...interface{}) *Status {
	return New(c, fmt.Sprintf(format, p...))
}

// FromProto returns a Status representing s.
func FromProto(s *pb.Status) *Status {
	return &Status{proto.Clone(s).(*pb.Status)}
}

// Code returns the status code contained in s.
func (s *Status) Code() codes.Code {
	if s == nil || s.Status == nil {
		return codes.OK
	}
	return codes.Code(s.GetCode())
}

// Err returns an immutable error representing s; returns nil if s.Code() is OK.
func (s *Status) Err() error {
	if s.Code() == codes.OK {
		return nil
	}
	return &Error{e: s.Proto()}
}

// Error wraps a pointer of a status proto. It implements error and Status,
// and a nil *Error should never be returned by this package.
type Error struct {
	e *pb.Status
}

// Proto returns s's status as an spb.Status proto message.
func (s *Status) Proto() *pb.Status {
	if s == nil {
		return nil
	}
	return proto.Clone(s.Status).(*pb.Status)
}

func (e *Error) Error() string {
	return fmt.Sprintf("rpc error: code = %s desc = %s", codes.Code(e.e.GetCode()), e.e.GetMessage())
}

// WithDetails returns a new status with the provided details messages appended to the status.
// If any errors are encountered, it returns nil and the first error encountered.
func (s *Status) WithDetails(details ...proto.Message) (*Status, error) {
	if s.Code() == codes.OK {
		return nil, errors.New("no error details for status with code OK")
	}
	// s.Code() != OK implies that s.Proto() != nil.
	p := s.Proto()
	for _, detail := range details {
		any, err := ptypes.MarshalAny(detail)
		if err != nil {
			return nil, err
		}
		p.Details = append(p.Details, any)
	}
	return &Status{Status: p}, nil
}

// Err returns an error representing c and msg.  If c is OK, returns nil.
func Err(c codes.Code, msg string) error {
	return New(c, msg).Err()
}

// FromError returns a Status representing err if it was produced from this
// package or has a method `GRPCStatus() *Status`. Otherwise, ok is false and a
// Status is returned with codes.Unknown and the original error message.
func FromError(err error) (s *Status, ok bool) {
	if err == nil {
		return nil, true
	}
	if se, ok := err.(interface {
		GRPCStatus() *Status
	}); ok {
		return se.GRPCStatus(), true
	}
	return New(codes.Unknown, err.Error()), false
}

// GRPCStatus returns the Status represented by se.
func (e *Error) GRPCStatus() *Status {
	return FromProto(e.e)
}

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return Err(c, fmt.Sprintf(format, a...))
}
