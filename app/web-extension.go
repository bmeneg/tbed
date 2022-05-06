package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"unsafe"
)

// textMaxLen is the maximum plaintext length the app can send to the
// extension. 1048576 is the specified limit for the payload, which includes
// 4 bytes of header and 4 bytes for each additional char added in the JSON
// data. Because of that, we decrease this plaintext limit to, at least,
// half, avoiding any surprises during serialization.
const textMaxLen = 524288

// tbedHeader is a custom header we use to send and receive control messages.
const tbedHeader = "--tbed-hdr"

var nativeEndian binary.ByteOrder

func init() {
	// Each native message is serialized starting with a 32 unisgned int
	// encoded in native order representing the message length. And, because
	// of that, we need to, first, get what's the actual machine's byte order.
	//
	// Note: this code was borrowed from TensorFlow project old times.
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		nativeEndian = binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		nativeEndian = binary.BigEndian
	default:
		panic("Could not determine native endianness.")
	}
}

// Connection holds the in and out stream pointers.
type Connection struct {
	in  *bufio.Reader
	out *bufio.Writer
}

// initConnection instantiates both in and out streams and return a connection
// instance, which should not be changed throughout app's execution.
func initConnection() Connection {
	con := Connection{}
	con.in = bufio.NewReader(os.Stdin)
	con.out = bufio.NewWriter(os.Stdout)
	return con
}

// Message hold attributes for both received and to-be-sent messages.
type Message struct {
	payload []string // JSON encoded message
	pages   uint32   // number of payload pages
}

func (msg *Message) setPayload(plaintext string) error {
	msg.pages = uint32(math.Ceil(float64(len(plaintext)) / textMaxLen))

	if msg.pages == 0 {
		return fmt.Errorf("payload with 0 pages")
	}

	if msg.pages == 1 {
		encoded, err := json.Marshal(plaintext)
		if err != nil {
			return err
		}
		msg.payload = append(msg.payload, string(encoded))
	} else {
		var (
			page   uint32
			offset uint32
		)

		// Adds the message containing the custom Pages header as the first
		// message payload to be sent.
		payload := fmt.Sprintf("%s\nPages: %d", tbedHeader, msg.pages)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		msg.payload = append(msg.payload, string(encoded))

		for page = 1; page <= msg.pages; page++ {
			payload = plaintext[offset:]
			length := uint32(len(payload))

			if length > textMaxLen {
				payload = payload[:page*textMaxLen]
				offset = page * textMaxLen
			}

			encoded, err = json.Marshal(payload)
			if err != nil {
				return err
			}
			msg.payload = append(msg.payload, string(encoded))
		}
	}

	dbg(fmt.Sprintf("msg pages: %d", msg.pages))
	dbg(fmt.Sprintf("msg payload: %s", msg.payload))

	return nil
}

// readMessage handles the incoming message and returns a new message pointer.
func (c Connection) readMessage() (*Message, error) {
	msg := &Message{}

	// Read message length from message header (uint32).
	header := make([]byte, 4)
	if _, err := io.ReadFull(c.in, header); err != nil {
		return nil, fmt.Errorf("failed to read message length")
	}
	dbg(fmt.Sprintf("read: message len raw: %x", header))

	length := nativeEndian.Uint32(header)
	dbg(fmt.Sprintf("read: message len decoded: %d", length))

	// Read encoded message from stdin based on the length header.
	encPayload := make([]byte, length)
	if _, err := io.ReadFull(c.in, encPayload); err != nil {
		return nil, err
	}
	if uint32(len(encPayload)) != length {
		return nil, fmt.Errorf(
			"received message length different from reported")
	}
	dbg(fmt.Sprintf("read: json message: %s", string(encPayload)))

	// Decode from JSON formatted stream
	var payload string
	if err := json.Unmarshal(encPayload, &payload); err != nil {
		return nil, err
	}
	msg.payload = append(msg.payload, payload)
	log.Printf("received message: %s", msg.payload)

	return msg, nil
}

// sendMessage send all the pages of a certain message to the extension.
func (c Connection) sendMessage(msg Message) error {
	var page uint32

	for page = 0; page < msg.pages; page++ {
		payload := msg.payload[page]
		length := uint32(len(payload))

		header := make([]byte, 4)
		nativeEndian.PutUint32(header, length)

		dbg(fmt.Sprintf("send: length: %d", length))
		dbg(fmt.Sprintf("send: payload: %s", payload))

		if _, err := c.out.Write(header); err != nil {
			return err
		}
		if _, err := c.out.Write([]byte(payload)); err != nil {
			return err
		}
		if err := c.out.Flush(); err != nil {
			return err
		}
		log.Printf("message sent: %s", payload)
	}
	return nil
}
