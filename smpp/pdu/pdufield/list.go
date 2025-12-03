// Copyright 2015 go-smpp authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pdufield

import (
	"bytes"
	"io"

	"github.com/florentchauveau/go-smpp/smpp/pdu/pdutext"
)

// List is a list of PDU fields.
type List []Name

// Decode decodes binary data in the given buffer to build a Map.
//
// If the ShortMessage field is present, and DataCoding as well,
// we attempt to decode text automatically. See pdutext package
// for more information.
func (l List) Decode(r *bytes.Buffer) (Map, error) {
	var (
		unsuccessCount, numDest, udhLength, smLength int
		dataCoding                                   pdutext.DataCoding
		udhiFlag                                     bool
	)
	f := make(Map)
loop:
	for _, k := range l {
		switch k {
		case
			AddressRange,
			DestinationAddr,
			ErrorCode,
			FinalDate,
			MessageID,
			MessageState,
			Password,
			ScheduleDeliveryTime,
			ServiceType,
			SourceAddr,
			SystemID,
			SystemType,
			ValidityPeriod:
			b, err := r.ReadBytes(0x00)
			if err == io.EOF {
				break loop
			}
			if err != nil {
				return nil, err
			}
			f[k] = &Variable{Data: b}
		case
			AddrNPI,
			AddrTON,
			DataCoding,
			DestAddrNPI,
			DestAddrTON,
			ESMClass,
			InterfaceVersion,
			NumberDests,
			NoUnsuccess,
			PriorityFlag,
			ProtocolID,
			RegisteredDelivery,
			ReplaceIfPresentFlag,
			SMDefaultMsgID,
			SourceAddrNPI,
			SourceAddrTON,
			SMLength:
			b, err := r.ReadByte()
			if err == io.EOF {
				break loop
			}
			if err != nil {
				return nil, err
			}
			f[k] = &Fixed{Data: b}
			switch k {
			case DataCoding:
				dataCoding = pdutext.DataCoding(b)
			case NoUnsuccess:
				unsuccessCount = int(b)
			case NumberDests:
				numDest = int(b)
			case SMLength:
				smLength = int(b)
			case ESMClass:
				mask := byte(1 << 6)
				udhiFlag = mask == b&mask
			}
		case UDHLength:
			if !udhiFlag {
				f[k] = &Null{}
				continue
			}
			b, err := r.ReadByte()
			if err == io.EOF {
				break loop
			}
			if err != nil {
				return nil, err
			}
			udhLength = int(b)
			f[k] = &Fixed{Data: b}
		case GSMUserData:
			if !udhiFlag {
				f[k] = &Null{}
				continue
			}
			var ieList []UDHIE
			var l int
			for i := udhLength; i > 0; i -= l + 2 {
				var ie UDHIE
				// Read IEI
				b, err := r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				ie.IEI = b
				// Read IELength
				b, err = r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				l = int(b)
				ie.IELength = b
				// Read IEData
				bt := r.Next(l)
				ie.IEData = bt
				ieList = append(ieList, ie)
				if len(bt) != l {
					break loop
				}
			}
			f[k] = &UDH{IE: ieList}
		case DestinationList:
			var destList []DestSme
			for i := 0; i < numDest; i++ {
				var dest DestSme
				// Read DestFlag
				b, err := r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				dest.Flag = Fixed{Data: b}
				// Read Ton
				b, err = r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				dest.Ton = Fixed{Data: b}
				// Read npi
				b, err = r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				dest.Npi = Fixed{Data: b}
				// Read address
				bt, err := r.ReadBytes(0x00)
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				dest.DestAddr = Variable{Data: bt}
				destList = append(destList, dest)
			}
			f[k] = &DestSmeList{Data: destList}
		case UnsuccessSme:
			var unsList []UnSme
			for i := 0; i < unsuccessCount; i++ {
				var uns UnSme
				// Read Ton
				b, err := r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				uns.Ton = Fixed{Data: b}
				// Read npi
				b, err = r.ReadByte()
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				uns.Npi = Fixed{Data: b}
				// Read address
				bt, err := r.ReadBytes(0x00)
				if err == io.EOF {
					break loop
				}
				if err != nil {
					return nil, err
				}
				uns.DestAddr = Variable{Data: bt}
				// Read error code
				uns.ErrCode = Variable{Data: r.Next(4)}
				// Add unSme to the list
				unsList = append(unsList, uns)
			}
			f[k] = &UnSmeList{Data: unsList}
		case ShortMessage:
			if udhiFlag {
				smLength -= udhLength + 1 // +1 for UDHLength octet
			}
			msg := r.Next(smLength)
			// Decode text according to DataCoding
			switch dataCoding {
			case pdutext.DefaultType:
				msg = pdutext.GSM7(msg).Decode()
			case pdutext.Latin1Type:
				msg = pdutext.Latin1(msg).Decode()
			case pdutext.UCS2Type:
				msg = pdutext.UCS2(msg).Decode()
			case pdutext.ISO88595Type:
				msg = pdutext.ISO88595(msg).Decode()
			}
			f[k] = &SM{Data: msg}
		}
	}
	return f, nil
}
