// Copyright 2024 The Erigon Authors
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.

package cbor

import (
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/ugorji/go/codec"
	"io"
	"math/big"
	"reflect"
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

func returnEncoderToPool(e *codec.Encoder) {
	select {
	case encoderPool <- e:
	default:
		logger.Trace("Allowing encoder to be garbage collected, pool is full")
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
