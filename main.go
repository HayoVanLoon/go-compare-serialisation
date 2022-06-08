// Copyright 2022 Hayo van Loon. All rights reserved.
// Use of this source code is governed by an Apache
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	pb "github.com/HayoVanLoon/genproto/research/serialisation"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
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

func generateFiles(lines int, fileJson, fileProto string) (err error) {
	closer := func(r io.ReadCloser) {
		if e := r.Close(); err == nil && e != nil {
			err = e
		}
	}
	fjson, err := os.Create(fileJson)
	if err != nil {
		log.Fatal("could not open output file ", fileJson)
	}
	defer closer(fjson)
	fproto, err := os.Create(fileProto)
	if err != nil {
		log.Fatal(err)
	}
	defer closer(fproto)

	sz := make([]byte, szBits)
	for i := 0; i < lines; i += 1 {
		inv := generateInvoice()

		bs, _ := protojson.Marshal(inv)
		if err != nil {
			return
		}
		if _, err = fjson.WriteString(string(bs) + "\n"); err != nil {
			return
		}

		bs, _ = proto.Marshal(inv)
		encodeInt32(uint32(len(bs)), sz)
		if _, err = fproto.Write(sz); err != nil {
			return
		}
		if _, err = fproto.Write(bs); err != nil {
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

func decodeJson(r io.Reader) (invs []*Invoice, err error) {
	dec := json.NewDecoder(r)
	for dec.More() {
		x := new(Invoice)
		err = dec.Decode(x)
		if err != nil {
			return
		}
		invs = append(invs, x)
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

func serialiseJson(invs []*Invoice) {
	for _, inv := range invs {
		_, _ = json.Marshal(inv)
	}
}

func serialiseProto(invs []*pb.Invoice) {
	for _, inv := range invs {
		_, _ = proto.Marshal(inv)
	}
}

func manipulatePlain(invs []*Invoice) {
	for _, inv := range invs {
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
	}
}

func manipulateProto(invs []*pb.Invoice) {
	for _, inv := range invs {
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
	}
}

const (
	fileJson  = "out/in.json"
	fileProto = "out/in.pb"
)

func main() {
	if err := generateFiles(10000, fileJson, fileProto); err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(fileJson)
	if err != nil {
		log.Fatal(err)
	}
	_, err = decodeJson(f)
	if err != nil {
		log.Fatal(err)
	}

	f, err = os.Open(fileProto)
	if err != nil {
		log.Fatal(err)
	}
	_, err = decodeProto(f)
	if err != nil {
		log.Fatal(err)
	}
}
