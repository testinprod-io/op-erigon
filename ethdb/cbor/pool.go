package cbor

import (
	"fmt"
	"io"
	"math/big"
	"reflect"

	"github.com/ledgerwatch/log/v3"
	"github.com/ugorji/go/codec"
)

var logger = log.New("package", "cbor")

// Pool of decoders
var decoderPool = make(chan *codec.Decoder, 128)

func Decoder(r io.Reader) *codec.Decoder {
	var d *codec.Decoder
	select {
	case d = <-decoderPool:
		d.Reset(r)
	default:
		{
			var handle codec.CborHandle
			handle.ReaderBufferSize = 64 * 1024
			handle.ZeroCopy = true // if you need access to object outside of db transaction - please copy bytes before deserialization
			handle.SetInterfaceExt(bigIntType, 1, BigIntExt{})
			handle.SetInterfaceExt(bigFloatType, 2, BigFloatExt{})
			d = codec.NewDecoder(r, &handle)
		}
	}
	return d
}

func DecoderBytes(r []byte) *codec.Decoder {
	var d *codec.Decoder
	select {
	case d = <-decoderPool:
		d.ResetBytes(r)
	default:
		{
			var handle codec.CborHandle
			handle.ReaderBufferSize = 64 * 1024
			handle.ZeroCopy = true // if you need access to object outside of db transaction - please copy bytes before deserialization
			handle.SetInterfaceExt(bigIntType, 1, BigIntExt{})
			handle.SetInterfaceExt(bigFloatType, 2, BigFloatExt{})
			d = codec.NewDecoderBytes(r, &handle)
		}
	}
	return d
}

func returnDecoderToPool(d *codec.Decoder) {
	select {
	case decoderPool <- d:
	default:
		logger.Trace("Allowing decoder to be garbage collected, pool is full")
	}
}

// Pool of encoders
var encoderPool = make(chan *codec.Encoder, 128)

func Encoder(w io.Writer) *codec.Encoder {
	var e *codec.Encoder
	select {
	case e = <-encoderPool:
		e.Reset(w)
	default:
		{
			var handle codec.CborHandle
			handle.WriterBufferSize = 64 * 1024
			handle.StructToArray = true
			handle.OptimumSize = true
			handle.StringToRaw = true
			handle.SetInterfaceExt(bigIntType, 1, BigIntExt{})
			handle.SetInterfaceExt(bigFloatType, 2, BigFloatExt{})

			e = codec.NewEncoder(w, &handle)
		}
	}
	return e
}

func EncoderBytes(w *[]byte) *codec.Encoder {
	var e *codec.Encoder
	select {
	case e = <-encoderPool:
		e.ResetBytes(w)
	default:
		{
			var handle codec.CborHandle
			handle.WriterBufferSize = 64 * 1024
			handle.StructToArray = true
			handle.OptimumSize = true
			handle.StringToRaw = true
			handle.SetInterfaceExt(bigIntType, 1, BigIntExt{})
			handle.SetInterfaceExt(bigFloatType, 2, BigFloatExt{})

			e = codec.NewEncoderBytes(w, &handle)
		}
	}
	return e
}

func returnEncoderToPool(e *codec.Encoder) {
	select {
	case encoderPool <- e:
	default:
		logger.Trace("Allowing encoder to be garbage collected, pool is full")
	}
}

func Return(d interface{}) {
	switch toReturn := d.(type) {
	case *codec.Decoder:
		returnDecoderToPool(toReturn)
	case *codec.Encoder:
		returnEncoderToPool(toReturn)
	default:
		panic(fmt.Sprintf("unexpected type: %T", d))
	}
}

var bigIntType = reflect.TypeOf(big.NewInt(0))
var bigFloatType = reflect.TypeOf(big.NewFloat(0))

type BigIntExt struct{}

func (x BigIntExt) ConvertExt(v interface{}) interface{} {
	v2 := v.(*big.Int)
	return v2.Bytes()
}
func (x BigIntExt) UpdateExt(dest interface{}, v interface{}) {
	d := dest.(*big.Int)
	d.SetBytes(v.([]byte))
}

type BigFloatExt struct{}

func (x BigFloatExt) ConvertExt(v interface{}) interface{} {
	v2 := v.(*big.Float)
	return v2.String()
}
func (x BigFloatExt) UpdateExt(dest interface{}, v interface{}) {
	d := dest.(*big.Float)
	d.SetString(v.(string))
}
