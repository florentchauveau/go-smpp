// Copyright 2015 go-smpp authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smpp

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/florentchauveau/go-smpp/smpp/pdu"
	"github.com/florentchauveau/go-smpp/smpp/pdu/pdufield"
	"github.com/florentchauveau/go-smpp/smpp/pdu/pdutext"
	"github.com/florentchauveau/go-smpp/smpp/smpptest"
)

func TestShortMessage(t *testing.T) {
	s := smpptest.NewUnstartedServer()
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		switch p.Header().ID {
		case pdu.SubmitSMID:
			r := pdu.NewSubmitSMResp()
			r.Header().Seq = p.Header().Seq
			_ = r.Fields().Set(pdufield.MessageID, "foobar")
			_ = c.Write(r)
		default:
			smpptest.EchoHandler(c, p)
		}
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:        s.Addr(),
		User:        smpptest.DefaultUser,
		Passwd:      smpptest.DefaultPasswd,
		RateLimiter: rate.NewLimiter(rate.Limit(10), 1),
	}
	defer tx.Close()
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	sm, err := tx.Submit(&ShortMessage{
		Src:      "root",
		Dst:      "foobar",
		Text:     pdutext.Raw("Lorem ipsum"),
		Validity: 10 * time.Minute,
		Register: pdufield.NoDeliveryReceipt,
	})
	if err != nil {
		t.Fatal(err)
	}
	msgid := sm.RespID()
	if msgid == "" {
		t.Fatalf("pdu does not contain msgid: %#v", sm.Resp())
	}
	if msgid != "foobar" {
		t.Fatalf("unexpected msgid: want foobar, have %q", msgid)
	}
}

func TestShortMessageWindowSize(t *testing.T) {
	s := smpptest.NewUnstartedServer()
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		time.Sleep(200 * time.Millisecond)
		r := pdu.NewSubmitSMResp()
		r.Header().Seq = p.Header().Seq
		_ = r.Fields().Set(pdufield.MessageID, "foobar")
		_ = c.Write(r)
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:        s.Addr(),
		User:        smpptest.DefaultUser,
		Passwd:      smpptest.DefaultPasswd,
		WindowSize:  2,
		RespTimeout: time.Second,
	}
	defer tx.Close()
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	msgc := make(chan *ShortMessage, 3)
	defer close(msgc)
	errc := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func(msgc chan *ShortMessage, errc chan error) {
			m := <-msgc
			if m == nil {
				return
			}
			_, err := tx.Submit(m)
			errc <- err
		}(msgc, errc)
		msgc <- &ShortMessage{
			Src:      "root",
			Dst:      "foobar",
			Text:     pdutext.Raw("Lorem ipsum"),
			Validity: 10 * time.Minute,
			Register: pdufield.NoDeliveryReceipt,
		}
	}
	nerr := 0
	for i := 0; i < 3; i++ {
		if <-errc == ErrMaxWindowSize {
			nerr++
		}
	}
	if nerr != 1 {
		t.Fatalf("unexpected # of errors. want 1, have %d", nerr)
	}
}

func TestLongMessage(t *testing.T) {
	s := smpptest.NewUnstartedServer()
	count := 0
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		switch p.Header().ID {
		case pdu.SubmitSMID:
			r := pdu.NewSubmitSMResp()
			r.Header().Seq = p.Header().Seq
			_ = r.Fields().Set(pdufield.MessageID, fmt.Sprintf("foobar%d", count))
			count++
			_ = c.Write(r)
		default:
			smpptest.EchoHandler(c, p)
		}
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:   s.Addr(),
		User:   smpptest.DefaultUser,
		Passwd: smpptest.DefaultPasswd,
	}
	defer tx.Close()
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	parts, err := tx.SubmitLongMsg(&ShortMessage{
		Src:      "root",
		Dst:      "foobar",
		Text:     pdutext.Raw("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nam consequat nisl enim, vel finibus neque aliquet sit amet. Interdum et malesuada fames ac ante ipsum primis in faucibus."),
		Validity: 10 * time.Minute,
		Register: pdufield.NoDeliveryReceipt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected %d responses, but received %d", 2, len(parts))
	}
	for index := range parts {
		msgid := parts[index].RespID()
		if msgid == "" {
			t.Fatalf("pdu does not contain msgid: %#v", parts[index].Resp())
		}
		if msgid != fmt.Sprintf("foobar%d", index) {
			t.Fatalf("unexpected msgid: want foobar%d, have %q", index, msgid)
		}
	}
}

func TestLongMessageEncode(t *testing.T) {
	sm := &ShortMessage{
		Src:      "root",
		Dst:      "foobar",
		Text:     pdutext.GSM7("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nam consequat nisl enim, vel finibus neque aliquet sit amet. Interdum et malesuada fames ac ante ipsum primis in faucibus."),
		Validity: 10 * time.Minute,
		Register: pdufield.NoDeliveryReceipt,
	}

	maxLen := 133 // 140-7 (UDH with 2 byte reference number)
	switch sm.Text.(type) {
	case pdutext.GSM7:
		maxLen = 152 // to avoid an escape character being split between payloads
	case pdutext.GSM7Packed:
		maxLen = 132 // to avoid an escape character being split between payloads
	case pdutext.UCS2:
		maxLen = 132 // to avoid a character being split between payloads
	}
	rawMsg := sm.Text.Encode()
	countParts := int((len(rawMsg)-1)/maxLen) + 1
	ref := uint16(rand.IntN(0xFFFF))
	for i := range countParts {
		udh := pdufield.NewUDHConcatenatedShortMessage(ref, countParts, i+1)
		p := pdu.NewSubmitSM(sm.TLVFields)
		f := p.Fields()
		_ = f.Set(pdufield.SourceAddr, sm.Src)
		_ = f.Set(pdufield.DestinationAddr, sm.Dst)
		if i != countParts-1 {
			_ = f.Set(pdufield.ShortMessage, pdutext.Raw(rawMsg[i*maxLen:(i+1)*maxLen]))
		} else {
			_ = f.Set(pdufield.ShortMessage, pdutext.Raw(rawMsg[i*maxLen:]))
		}
		_ = f.Set(pdufield.RegisteredDelivery, uint8(sm.Register))
		if sm.Validity != 0 {
			_ = f.Set(pdufield.ValidityPeriod, convertValidity(sm.Validity))
		}
		_ = f.Set(pdufield.ServiceType, sm.ServiceType)
		_ = f.Set(pdufield.SourceAddrTON, sm.SourceAddrTON)
		_ = f.Set(pdufield.SourceAddrNPI, sm.SourceAddrNPI)
		_ = f.Set(pdufield.DestAddrTON, sm.DestAddrTON)
		_ = f.Set(pdufield.DestAddrNPI, sm.DestAddrNPI)
		_ = f.Set(pdufield.ESMClass, pdufield.ESMClassUDHIndicator)
		_ = f.Set(pdufield.ProtocolID, sm.ProtocolID)
		_ = f.Set(pdufield.PriorityFlag, sm.PriorityFlag)
		_ = f.Set(pdufield.ScheduleDeliveryTime, sm.ScheduleDeliveryTime)
		_ = f.Set(pdufield.ReplaceIfPresentFlag, sm.ReplaceIfPresentFlag)
		_ = f.Set(pdufield.SMDefaultMsgID, sm.SMDefaultMsgID)
		_ = f.Set(pdufield.DataCoding, uint8(sm.Text.Type()))
		_ = f.Set(pdufield.UDHLength, uint8(udh.Len()))
		_ = f.Set(pdufield.GSMUserData, &udh)
		_ = f.Set(pdufield.SMLength, uint8(f[pdufield.ShortMessage].Len()+udh.Len()+1)) // +1 for UDHLength octet
		wire := bytes.NewBuffer(nil)
		err := p.SerializeTo(wire)
		if err != nil {
			t.Fatalf("error marshalling pdu: %v", err)
		}
		encoded, err := pdu.Decode(wire)
		if err != nil {
			t.Fatalf("error unmarshalling pdu: %v", err)
		}
		encodedSM := encoded.Fields()[pdufield.ShortMessage]
		if encodedSM.String() != f[pdufield.ShortMessage].String() {
			t.Fatalf("part %d: expected short message %q, got %q", i+1, f[pdufield.ShortMessage].String(), encodedSM.String())
		}
		// encoded.UDH()
		udhField := encoded.UDH()
		if udhField == nil {
			t.Fatalf("part %d: missing UDH field", i+1)
		}
		concatenated, expectedRef, total, part := udhField.IsConcatenated()
		if !concatenated {
			t.Fatalf("part %d: UDH IsConcatenated = %v, want %v", i+1, concatenated, true)
		}
		if total != countParts {
			t.Fatalf("part %d: UDH IsConcatenated total = %d, want %d", i+1, total, countParts)
		}
		if part != i+1 {
			t.Fatalf("part %d: UDH IsConcatenated part = %d, want %d", i+1, part, i+1)
		}
		if expectedRef != int(ref) {
			t.Fatalf("part %d: UDH IsConcatenated ref = %d, want %d", i+1, expectedRef, ref)
		}
	}
}

func TestLongMessageAsUCS2(t *testing.T) {
	s := smpptest.NewUnstartedServer()
	var receivedMsg string
	shortMsg := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nam consequat nisl enim, vel finibus neque aliquet sit amet. Interdum et malesuada fames ac ante ipsum primis in faucibus. ✓"
	count := 0
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		switch p.Header().ID {
		case pdu.SubmitSMID:
			r := pdu.NewSubmitSMResp()
			r.Header().Seq = p.Header().Seq
			_ = r.Fields().Set(pdufield.MessageID, fmt.Sprintf("foobar%d", count))
			count++
			receivedMsg = receivedMsg + p.Fields()[pdufield.ShortMessage].String()
			_ = c.Write(r)
		default:
			smpptest.EchoHandler(c, p)
		}
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:   s.Addr(),
		User:   smpptest.DefaultUser,
		Passwd: smpptest.DefaultPasswd,
	}
	defer tx.Close()
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	parts, err := tx.SubmitLongMsg(&ShortMessage{
		Src:      "root",
		Dst:      "foobar",
		Text:     pdutext.UCS2(shortMsg),
		Validity: 10 * time.Minute,
		Register: pdufield.NoDeliveryReceipt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("expected %d responses, but received %d", 3, len(parts))
	}
	for index := range parts {
		msgid := parts[index].RespID()
		if msgid == "" {
			t.Fatalf("pdu does not contain msgid: %#v", parts[index].Resp())
		}

		if receivedMsg != shortMsg {
			t.Fatalf("receivedMsg: %v, does not match shortMsg: %v", receivedMsg, shortMsg)
		}

		if msgid != fmt.Sprintf("foobar%d", index) {
			t.Fatalf("unexpected msgid: want foobar%d, have %q", index, msgid)
		}
	}
}

func TestQuerySM(t *testing.T) {
	s := smpptest.NewUnstartedServer()
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		r := pdu.NewQuerySMResp()
		r.Header().Seq = p.Header().Seq
		_ = r.Fields().Set(pdufield.MessageID, p.Fields()[pdufield.MessageID])
		_ = r.Fields().Set(pdufield.MessageState, 2)
		_ = c.Write(r)
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:   s.Addr(),
		User:   smpptest.DefaultUser,
		Passwd: smpptest.DefaultPasswd,
	}
	defer tx.Close()
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	qr, err := tx.QuerySM("root", "13", uint8(5), uint8(0))
	if err != nil {
		t.Fatal(err)
	}
	if qr.MsgID != "13" {
		t.Fatalf("unexpected msgid: want 13, have %s", qr.MsgID)
	}
	if qr.MsgState != "DELIVERED" {
		t.Fatalf("unexpected state: want DELIVERED, have %q", qr.MsgState)
	}
}

func TestSubmitMulti(t *testing.T) {
	//construct a byte array with the UnsuccessSme
	var bArray []byte
	bArray = append(bArray, byte(0x00))       // TON
	bArray = append(bArray, byte(0x00))       // NPI
	bArray = append(bArray, []byte("123")...) // Address
	bArray = append(bArray, byte(0x00))       // Error
	bArray = append(bArray, byte(0x00))       // Error
	bArray = append(bArray, byte(0x00))       // Error
	bArray = append(bArray, byte(0x11))       // Error
	bArray = append(bArray, byte(0x00))       // null terminator

	s := smpptest.NewUnstartedServer()
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		switch p.Header().ID {
		case pdu.SubmitMultiID:
			r := pdu.NewSubmitMultiResp()
			r.Header().Seq = p.Header().Seq
			_ = r.Fields().Set(pdufield.MessageID, "foobar")
			_ = r.Fields().Set(pdufield.NoUnsuccess, uint8(1))
			_ = r.Fields().Set(pdufield.UnsuccessSme, bArray)
			_ = c.Write(r)
		default:
			smpptest.EchoHandler(c, p)
		}
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:   s.Addr(),
		User:   smpptest.DefaultUser,
		Passwd: smpptest.DefaultPasswd,
	}
	defer tx.Close()
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	sm, err := tx.Submit(&ShortMessage{
		Src:      "root",
		DstList:  []string{"123", "2233", "32322", "4234234"},
		DLs:      []string{"DistributionList1"},
		Text:     pdutext.Raw("Lorem ipsum"),
		Validity: 10 * time.Minute,
		Register: pdufield.NoDeliveryReceipt,
	})
	if err != nil {
		t.Fatal(err)
	}
	msgid := sm.RespID()
	if msgid == "" {
		t.Fatalf("pdu does not contain msgid: %#v", sm.Resp())
	}
	if msgid != "foobar" {
		t.Fatalf("unexpected msgid: want foobar, have %q", msgid)
	}
	noUncess, _ := sm.NumbUnsuccess()
	if noUncess != 1 {
		t.Fatalf("unexpected number of unsuccess %d", noUncess)
	}
	uncessSmes, _ := sm.UnsuccessSmes()
	if len(uncessSmes) != 1 {
		t.Fatalf("unsucess sme list should have a size of 1, has %d", len(uncessSmes))
	}
}

func TestNotConnected(t *testing.T) {
	s := smpptest.NewUnstartedServer()
	s.Handler = func(c smpptest.Conn, p pdu.Body) {
		switch p.Header().ID {
		case pdu.SubmitSMID:
			r := pdu.NewSubmitSMResp()
			r.Header().Seq = p.Header().Seq
			_ = r.Fields().Set(pdufield.MessageID, "foobar")
			_ = c.Write(r)
		default:
			smpptest.EchoHandler(c, p)
		}
	}
	s.Start()
	defer s.Close()
	tx := &Transmitter{
		Addr:   s.Addr(),
		User:   smpptest.DefaultUser,
		Passwd: smpptest.DefaultPasswd,
	}
	// Open connection and then close it
	conn := <-tx.Bind()
	switch conn.Status() {
	case Connected:
	default:
		t.Fatal(conn.Error())
	}
	tx.Close()
	_, err := tx.Submit(&ShortMessage{
		Src:      "root",
		Dst:      "foobar",
		Text:     pdutext.Raw("Lorem ipsum"),
		Validity: 10 * time.Minute,
		Register: pdufield.NoDeliveryReceipt,
	})
	if err != ErrNotConnected {
		t.Fatalf("Error should be not connect, got %s", err.Error())
	}

}
