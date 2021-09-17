package log

import (
	"github.com/go-errors/errors"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"time"
)

//Custom encoder to use go-errors traces to improve debugging
//When the following issue is resolved this shouldn't be necessary
//https://github.com/uber-go/zap/issues/514

type xjoinEncoder struct {
	encoder zapcore.Encoder
}

func (enc *xjoinEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	if ent.Stack != "" {
		for _, field := range fields {
			if field.Key == "error" {
				_, ok := field.Interface.(*errors.Error)
				if ok {
					ent.Stack = field.Interface.(*errors.Error).ErrorStack()
				}
			}
		}
	}
	return enc.encoder.EncodeEntry(ent, fields)
}

func (enc *xjoinEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	return enc.encoder.AddArray(key, marshaler)
}

func (enc *xjoinEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	return enc.encoder.AddObject(key, marshaler)
}

func (enc *xjoinEncoder) AddBinary(key string, value []byte) {
	enc.encoder.AddBinary(key, value)
}

func (enc *xjoinEncoder) AddByteString(key string, value []byte) {
	enc.encoder.AddByteString(key, value)
}

func (enc *xjoinEncoder) AddBool(key string, value bool) {
	enc.encoder.AddBool(key, value)
}

func (enc *xjoinEncoder) AddComplex128(key string, value complex128) {
	enc.encoder.AddComplex128(key, value)
}

func (enc *xjoinEncoder) AddComplex64(key string, value complex64) {
	enc.encoder.AddComplex64(key, value)
}

func (enc *xjoinEncoder) AddDuration(key string, value time.Duration) {
	enc.encoder.AddDuration(key, value)
}

func (enc *xjoinEncoder) AddFloat64(key string, value float64) {
	enc.encoder.AddFloat64(key, value)
}

func (enc *xjoinEncoder) AddFloat32(key string, value float32) {
	enc.encoder.AddFloat32(key, value)
}

func (enc *xjoinEncoder) AddInt(key string, value int) {
	enc.encoder.AddInt(key, value)
}

func (enc *xjoinEncoder) AddInt64(key string, value int64) {
	enc.encoder.AddInt64(key, value)
}

func (enc *xjoinEncoder) AddInt32(key string, value int32) {
	enc.encoder.AddInt32(key, value)
}

func (enc *xjoinEncoder) AddInt16(key string, value int16) {
	enc.encoder.AddInt16(key, value)
}

func (enc *xjoinEncoder) AddInt8(key string, value int8) {
	enc.encoder.AddInt8(key, value)
}

func (enc *xjoinEncoder) AddString(key, value string) {
	enc.encoder.AddString(key, value)
}

func (enc *xjoinEncoder) AddTime(key string, value time.Time) {
	enc.encoder.AddTime(key, value)
}

func (enc *xjoinEncoder) AddUint(key string, value uint) {
	enc.encoder.AddUint(key, value)
}

func (enc *xjoinEncoder) AddUint64(key string, value uint64) {
	enc.encoder.AddUint64(key, value)
}

func (enc *xjoinEncoder) AddUint32(key string, value uint32) {
	enc.encoder.AddUint32(key, value)
}

func (enc *xjoinEncoder) AddUint16(key string, value uint16) {
	enc.encoder.AddUint16(key, value)
}

func (enc *xjoinEncoder) AddUint8(key string, value uint8) {
	enc.encoder.AddUint8(key, value)
}

func (enc *xjoinEncoder) AddUintptr(key string, value uintptr) {
	enc.encoder.AddUintptr(key, value)
}

func (enc *xjoinEncoder) AddReflected(key string, value interface{}) error {
	return enc.encoder.AddReflected(key, value)
}

func (enc *xjoinEncoder) OpenNamespace(key string) {
	enc.encoder.OpenNamespace(key)
}

func (enc *xjoinEncoder) Clone() zapcore.Encoder {
	return &xjoinEncoder{enc.encoder.Clone()}
}
