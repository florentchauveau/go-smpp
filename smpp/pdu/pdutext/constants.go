package pdutext

const (
	// MaxGSM7ShortMessageLenEncoded is the maximum length of a GSM 7-bit
	// encoded short message without UDH.
	MaxGSM7ShortMessageLenEncoded = 160

	// MaxUCS2ShortMessageLenEncoded is the maximum length of a UCS2 encoded
	// short message without UDH.
	MaxUCS2ShortMessageLenEncoded = 140

	// MaxConcatenatedShortMessageLenEncoded is the maximum length of a concatenated
	// short message part payload.
	MaxConcatenatedShortMessageLenEncoded = 133 // 140 - 7 (UDH with 2 byte reference number)

	// MaxGSM7ConcatenatedShortMessageLenEncoded is the maximum length of a GSM 7-bit
	// encoded concatenated short message part payload.
	MaxGSM7ConcatenatedShortMessageLenEncoded = 152 // 160 - 7 (UDH with 2 byte reference number) -1 to avoid an escape character being split between payloads

	// MaxUCS2ConcatenatedShortMessageLenEncoded is the maximum length of a UCS2
	// encoded concatenated short message part payload.
	MaxUCS2ConcatenatedShortMessageLenEncoded = 132 // 140 - 7 (UDH with 2 byte reference number) -1 to avoid a character being split between payloads
)
