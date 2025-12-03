// Copyright 2015 go-smpp authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pdufield

import "io"

// Body is an interface for manipulating binary PDU field data.
type Body interface {
	Len() int
	Raw() any
	String() string
	Bytes() []byte
	SerializeTo(w io.Writer) error
}

// New parses the given binary data and returns a Data object,
// or nil if the field Name is unknown.
func New(n Name, data []byte) Body {
	switch n {
	case
		AddrNPI,
		AddrTON,
		DataCoding,
		DestAddrNPI,
		DestAddrTON,
		ESMClass,
		ErrorCode,
		InterfaceVersion,
		MessageState,
		NumberDests,
		NoUnsuccess,
		PriorityFlag,
		ProtocolID,
		RegisteredDelivery,
		ReplaceIfPresentFlag,
		SMDefaultMsgID,
		SMLength,
		SourceAddrNPI,
		SourceAddrTON:
		if data == nil {
			data = []byte{0}
		}
		return &Fixed{Data: data[0]}
	case
		AddressRange,
		DestinationAddr,
		DestinationList,
		FinalDate,
		MessageID,
		Password,
		ScheduleDeliveryTime,
		ServiceType,
		SourceAddr,
		SystemID,
		SystemType,
		UnsuccessSme,
		ValidityPeriod:
		if data == nil {
			data = []byte{}
		}
		return &Variable{Data: data}
	case UDHLength:
		if len(data) == 0 {
			return &Null{}
		}
		return &Fixed{Data: data[0]}
	case ShortMessage:
		if data == nil {
			data = []byte{}
		}
		return &SM{Data: data}
	case GSMUserData:
		if len(data) > 2 {
			udh := []UDHIE{}
			for i := 0; i < len(data); {
				dataLength := int(data[i+1])
				ie := UDHIE{
					IEI:      data[i],
					IELength: data[i+1],
					IEData:   data[i+2 : i+2+dataLength],
				}
				udh = append(udh, ie)
				i += 2 + dataLength
			}
			return &UDH{IE: udh}
		}
		return &Null{}
	default:
		return nil
	}
}
