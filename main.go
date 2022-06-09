// Copyright 2022 Hayo van Loon. All rights reserved.
// Use of this source code is governed by an Apache
// license that can be found in the LICENSE file.

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	pb "github.com/HayoVanLoon/genproto/research/serialisation"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"unicode/utf8"
)

type Invoice struct {
	Invoicee     string        `json:"invoicee"`
	Address      Address       `json:"address"`
	InvoiceLines []InvoiceLine `json:"invoiceLines"`
	Subtotal     float32       `json:"subtotal"`
	TaxPct       float32       `json:"taxPct"`
	Total        float32       `json:"total"`
}

type InvoiceLine struct {
	ProductName string  `json:"productName"`
	Price       float32 `json:"price"`
	Quantity    int32   `json:"quantity"`
}

type Address struct {
	HouseNumber int32  `json:"houseNumber"`
	Street      string `json:"street"`
	PostalCode  string `json:"postalCode"`
	Country     string `json:"country"`
}

func randAscii(l int) string {
	b := strings.Builder{}
	for i := 0; i < l; i += 1 {
		b.WriteRune(randAsciiLetter())
	}
	return b.String()
}

func randAsciiLetter() rune {
	const (
		base      = 'A'
		exclStart = '['
		exclEnd   = '`' + 1
		exclLen   = exclEnd - exclStart
		rng       = 'z' - 'A' - exclLen
	)
	r := base + rand.Intn(rng)
	if exclStart <= r && r <= exclEnd {
		// jump over non-letter runes
		r += exclLen
	}
	return rune(r)
}

func generateInvoice() *pb.Invoice {
	in := new(pb.Invoice)
	in.Invoicee = randAscii(8 + rand.Intn(24))
	in.Address = new(pb.Address)
	in.Address.HouseNumber = int32(1 + rand.Intn(500))
	in.Address.Street = randAscii(4 + rand.Intn(12))
	in.Address.PostalCode = randAscii(6)
	in.Address.Country = randAscii(4 + rand.Intn(12))
	var subtotal float32
	lines := 1 + rand.Intn(9)
	for i := 0; i < lines; i += 1 {
		inv := &pb.Invoice_InvoiceLine{
			ProductName: randAscii(4 + rand.Intn(24)),
			Price:       float32(rand.Intn(100)) + float32(rand.Intn(100))*.01,
			Quantity:    int32(1 + rand.Intn(99)),
		}
		in.InvoiceLines = append(in.InvoiceLines, inv)
		subtotal += inv.Price * float32(inv.Quantity)
	}
	in.Subtotal = subtotal
	in.TaxPct = float32(rand.Intn(33)) / 100
	in.Total = subtotal * (1 + in.TaxPct)
	return in
}

// number of bits for protobuf message size field
const szBits = 4

const delim = "\n"

func generateFiles(lines int, filePrefix, protoDelim string) (err error) {
	closer := func(r io.ReadCloser) {
		if e := r.Close(); err == nil && e != nil {
			err = e
		}
	}
	if protoDelim == "" {
		protoDelim = delim
	}
	fj, err := os.Create(filePrefix + fileExtJson)
	if err != nil {
		return err
	}
	defer closer(fj)
	fp, err := os.Create(filePrefix + fileExtProto)
	if err != nil {
		return err
	}
	defer closer(fp)
	fps, err := os.Create(filePrefix + fileExtProtoString)
	if err != nil {
		return err
	}
	defer closer(fps)

	sz := make([]byte, szBits)
	for i := 0; i < lines; i += 1 {
		inv := generateInvoice()

		// json
		bs, _ := protojson.Marshal(inv)
		if err != nil {
			return
		}
		if _, err = fj.WriteString(string(bs) + delim); err != nil {
			return
		}

		// proto bytes
		bs, _ = proto.Marshal(inv)
		encodeInt32(uint32(len(bs)), sz)
		if _, err = fp.Write(sz); err != nil {
			return
		}
		if _, err = fp.Write(bs); err != nil {
			return
		}

		// proto string
		b64 := base64.RawStdEncoding.EncodeToString(bs)
		if err != nil {
			return fmt.Errorf("error encoding to proto string: %s", err)
		}
		if _, err = fps.WriteString(b64 + protoDelim); err != nil {
			return
		}
	}
	return nil
}

func encodeInt32(u uint32, p []byte) {
	for i := 0; i < szBits; i += 1 {
		p[szBits-i-1] = uint8(u >> (i * 8))
	}
}

func decodeInt32(p []byte) uint32 {
	var u uint32
	for i := 0; i < szBits; i += 1 {
		u += uint32(p[szBits-i-1]) << (i * 8)
	}
	return u
}

func decodeJson(r io.Reader) (invs []Invoice, err error) {
	dec := json.NewDecoder(r)
	for dec.More() {
		x := new(Invoice)
		err = dec.Decode(x)
		if err != nil {
			return
		}
		invs = append(invs, *x)
	}
	return
}

func decodeProto(r io.Reader) (invs []*pb.Invoice, err error) {
	sz := make([]byte, szBits)
	for {
		_, err := r.Read(sz)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		p := make([]byte, decodeInt32(sz))
		_, err = r.Read(p)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		inv := new(pb.Invoice)
		if err := proto.Unmarshal(p, inv); err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	return invs, nil
}

func decodeProtoString(rd io.Reader, protoDelim string) (invs []*pb.Invoice, err error) {
	if protoDelim == "" {
		protoDelim = delim
	}
	p := make([]byte, 128)
	rem := 0
	idx := 0
	b := &strings.Builder{}
	for {
		n, err := rd.Read(p[rem:])
		if err != nil && err != io.EOF {
			return nil, err
		}
		tot := n + rem
		i := 0
		for {
			if i == tot {
				rem = 0
				break
			}
			r, size := utf8.DecodeRune(p[i:tot])
			if r == utf8.RuneError {
				if size == 1 {
					rem = copy(p, p[i:tot])
				}
				if rem > utf8.UTFMax {
					return nil, fmt.Errorf("could not convert utf8 at %d: %v", idx, p[:utf8.UTFMax])
				}
				break
			}
			if string(r) == protoDelim {
				inv, err := unmarshalB64(b)
				if err != nil {
					return nil, err
				}
				invs = append(invs, inv)
			} else {
				b.WriteRune(r)
			}
			i += size
			idx += size
		}
		if err == io.EOF {
			break
		}
	}
	if b.Len() > 0 {
		inv, err := unmarshalB64(b)
		if err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	return invs, nil
}

func unmarshalB64(b *strings.Builder) (*pb.Invoice, error) {
	bs, err := base64.RawStdEncoding.DecodeString(b.String())
	if err != nil {
		return nil, fmt.Errorf("error decoding b64 message: %s", err)
	}
	inv := new(pb.Invoice)
	if err := proto.Unmarshal(bs, inv); err != nil {
		return nil, err
	}
	b.Reset()
	return inv, nil
}

func serialiseJson(invs []Invoice) {
	for _, inv := range invs {
		_, _ = json.Marshal(inv)
	}
}

func serialiseProto(invs []*pb.Invoice) {
	for _, inv := range invs {
		_, _ = proto.Marshal(inv)
	}
}

func serialiseProtoString(invs []*pb.Invoice) {
	for _, inv := range invs {
		bs, _ := proto.Marshal(inv)
		_ = base64.RawStdEncoding.EncodeToString(bs)
	}
}

func manipulatePlain(invs []Invoice) []Invoice {
	var updated []Invoice
	for _, orig := range invs {
		inv := orig
		inv.Invoicee = inv.Invoicee[1:] + string(randAsciiLetter())
		inv.Address.HouseNumber = -inv.Address.HouseNumber
		inv.Address.Street = inv.Address.Street[1:] + string(randAsciiLetter())
		inv.Address.PostalCode = inv.Address.PostalCode[1:] + string(randAsciiLetter())
		inv.Address.Country = inv.Address.Country[1:] + string(randAsciiLetter())
		var subtotal float32
		for i := range inv.InvoiceLines {
			inv.InvoiceLines[i].ProductName = inv.InvoiceLines[i].ProductName[1:] + string(randAsciiLetter())
			inv.InvoiceLines[i].Price = -inv.InvoiceLines[i].Price
			inv.InvoiceLines[i].Quantity = -inv.InvoiceLines[i].Quantity
			subtotal += inv.InvoiceLines[i].Price * float32(inv.InvoiceLines[i].Quantity)
		}
		inv.Subtotal = subtotal
		inv.TaxPct = -inv.TaxPct
		inv.Total = subtotal * (1 + inv.TaxPct)
		updated = append(updated, inv)
	}
	return updated
}

func manipulateProto(invs []*pb.Invoice, immutable bool) []*pb.Invoice {
	var updated []*pb.Invoice
	for _, orig := range invs {
		var inv *pb.Invoice
		if immutable {
			inv = new(pb.Invoice)
			proto.Merge(inv, orig)
		} else {
			inv = orig
		}
		inv.Invoicee = inv.Invoicee[1:] + string(randAsciiLetter())
		inv.Address.HouseNumber = -inv.Address.HouseNumber
		inv.Address.Street = inv.Address.Street[1:] + string(randAsciiLetter())
		inv.Address.PostalCode = inv.Address.PostalCode[1:] + string(randAsciiLetter())
		inv.Address.Country = inv.Address.Country[1:] + string(randAsciiLetter())
		var subtotal float32
		for i := range inv.InvoiceLines {
			inv.InvoiceLines[i].ProductName = inv.InvoiceLines[i].ProductName[1:] + string(randAsciiLetter())
			inv.InvoiceLines[i].Price = -inv.InvoiceLines[i].Price
			inv.InvoiceLines[i].Quantity = -inv.InvoiceLines[i].Quantity
			subtotal += inv.InvoiceLines[i].Price * float32(inv.InvoiceLines[i].Quantity)
		}
		inv.Subtotal = subtotal
		inv.TaxPct = -inv.TaxPct
		inv.Total = subtotal * (1 + inv.TaxPct)
		updated = append(updated, inv)
	}
	return updated
}

const (
	filePrefix         = "out/in"
	fileExtJson        = ".json"
	fileExtProto       = ".pb"
	fileExtProtoString = ".pb.txt"
)

func main() {
	protoDelim := ""
	if err := generateFiles(10000, filePrefix, protoDelim); err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(filePrefix + fileExtJson)
	if err != nil {
		log.Fatal(err)
	}
	_, err = decodeJson(f)
	if err != nil {
		log.Fatal(err)
	}

	f, err = os.Open(filePrefix + fileExtProto)
	if err != nil {
		log.Fatal(err)
	}
	_, err = decodeProto(f)
	if err != nil {
		log.Fatal(err)
	}

	f, err = os.Open(filePrefix + fileExtProtoString)
	if err != nil {
		log.Fatal(err)
	}
	_, err = decodeProtoString(f, protoDelim)
	if err != nil {
		log.Fatal(err)
	}
}
