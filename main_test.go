// Copyright 2022 Hayo van Loon. All rights reserved.
// Use of this source code is governed by an Apache
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/base64"
	pb "github.com/HayoVanLoon/genproto/research/serialisation"
	"google.golang.org/protobuf/proto"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"testing"
)

var bs, bs2, bs3 []byte
var decoded []Invoice
var decoded2 []*pb.Invoice

func init() {
	var err error
	if bs, err = ioutil.ReadFile(filePrefix + fileExtJson); err != nil {
		log.Fatal(err)
	}
	if bs2, err = ioutil.ReadFile(filePrefix + fileExtProto); err != nil {
		log.Fatal(err)
	}
	if bs3, err = ioutil.ReadFile(filePrefix + fileExtProtoString); err != nil {
		log.Fatal(err)
	}
	decoded, _ = decodeJson(bytes.NewReader(bs))
	decoded2, _ = decodeProto(bytes.NewReader(bs2))
}

func Benchmark_decode(b *testing.B) {
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
	b.Run("proto string", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			_, _ = decodeProtoString(bytes.NewReader(bs3), "")
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
	b.Run("proto string", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			serialiseProtoString(decoded2)
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
			manipulateProto(decoded2, false)
		}
	})
	b.Run("proto immutable", func(b *testing.B) {
		for i := 0; i < b.N; i += 1 {
			manipulateProto(decoded2, true)
		}
	})
}

func Test_decodeProtoString(t *testing.T) {
	inv1 := &pb.Invoice{Invoicee: "\uE001fred日本"}
	inv2 := &pb.Invoice{Invoicee: "\uE001日本john"}
	ser := func(i *pb.Invoice) string {
		bs, _ := proto.Marshal(i)
		return base64.RawStdEncoding.EncodeToString(bs)
	}
	genRd := func(d string, rep int) io.Reader {
		b := strings.Builder{}
		s1 := ser(inv1)
		s2 := ser(inv2)
		b.WriteString(s1)
		b.WriteString(d)
		b.WriteString(s2)
		for i := 1; i < rep; i += 1 {
			b.WriteString(d)
			b.WriteString(s1)
			b.WriteString(d)
			b.WriteString(s2)
		}
		return bytes.NewReader([]byte(b.String()))
	}
	want := func(size int) []*pb.Invoice {
		var xs []*pb.Invoice
		for i := 0; i < size; i += 1 {
			xs = append(xs, inv1, inv2)
		}
		return xs
	}

	type args struct {
		rd         io.Reader
		protoDelim string
	}
	tests := []struct {
		name     string
		args     args
		wantInvs []*pb.Invoice
		wantErr  bool
	}{
		{
			"simple",
			args{genRd("\n", 128), "\n"},
			want(128),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInvs, err := decodeProtoString(tt.args.rd, tt.args.protoDelim)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeProtoString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(gotInvs) != len(tt.wantInvs) {
				t.Errorf("unequal lengths: got %d, want %d", len(gotInvs), len(tt.wantInvs))
				return
			}
			for i := range gotInvs {
				if !proto.Equal(gotInvs[i], tt.wantInvs[i]) {
					t.Errorf("decodeProtoString() gotInvs[%d] = %v, want %v", i, gotInvs[i], tt.wantInvs[i])
				}
			}
		})
	}
}
