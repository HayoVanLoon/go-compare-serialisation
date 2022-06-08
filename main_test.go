// Copyright 2022 Hayo van Loon. All rights reserved.
// Use of this source code is governed by an Apache
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	pb "github.com/HayoVanLoon/genproto/research/serialisation"
	"io/ioutil"
	"log"
	"testing"
)

var bs, bs2 []byte
var decoded []*Invoice
var decoded2 []*pb.Invoice

func init() {
	var err error
	if bs, err = ioutil.ReadFile(fileJson); err != nil {
		log.Fatal(err)
	}
	if bs2, err = ioutil.ReadFile(fileProto); err != nil {
		log.Fatal(err)
	}
	decoded, _ = decodeJson(bytes.NewReader(bs))
	decoded2, _ = decodeProto(bytes.NewReader(bs2))
}

func Benchmark_decodeJson(b *testing.B) {
	b.Run("json", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			_, _ = decodeJson(bytes.NewReader(bs))
		}
	})
	b.Run("proto", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			_, _ = decodeProto(bytes.NewReader(bs2))
		}
	})
}

func Benchmark_serialise(b *testing.B) {
	b.Run("json", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			serialiseJson(decoded)
		}
	})
	b.Run("proto", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			serialiseProto(decoded2)
		}
	})
}

func Benchmark_manipulate(b *testing.B) {
	b.Run("plain", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			manipulatePlain(decoded)
		}
	})
	b.Run("proto", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			manipulateProto(decoded2)
		}
	})
}
