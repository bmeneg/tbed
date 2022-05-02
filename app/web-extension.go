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

// PayloadMaxLen is the maximum payload length the app can send to the
// extension.
const payloadMaxLen = 1048576

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
	payload string // JSON encoded message
	length  uint32 // encoded message length
	pages   uint32 // pages needed to send the msg considering payloadMaxLen
}

func (msg *Message) setPayload(payload string) error {
	encPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg.payload = string(encPayload)
	msg.length = uint32(len(encPayload))
	msg.pages = uint32(math.Ceil(float64(msg.length) / payloadMaxLen))

	dbg(fmt.Sprintf("msg length: %d", msg.length))
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

	msg.length = nativeEndian.Uint32(header)
	dbg(fmt.Sprintf("read: message len decoded: %d", msg.length))

	// Read encoded message from stdin based on the length header.
	encPayload := make([]byte, msg.length)
	if _, err := io.ReadFull(c.in, encPayload); err != nil {
		return nil, err
	}
	if uint32(len(encPayload)) != msg.length {
		return nil, fmt.Errorf(
			"received message length different from reported")
	}
	dbg(fmt.Sprintf("read: json message: %s", string(encPayload)))

	// Decode from JSON formatted stream
	if err := json.Unmarshal(encPayload, &msg.payload); err != nil {
		return nil, err
	}
	log.Printf("received message: %s", msg.payload)

	return msg, nil
}

// send sends messages to the other end of the connection considering
// paged messages.
func (c Connection) send(msg Message) error {
	var (
		page   uint32
		offset uint32
	)

	for page = 1; page <= msg.pages; page++ {
		payload := msg.payload[offset:msg.length]
		if msg.pages > 1 {
			payload = msg.payload[offset : page*payloadMaxLen]
			offset = page * payloadMaxLen
			dbg(fmt.Sprintf("send: page %d", page))
		}

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

// sendMessage wraps the generic send() call to first handle any custom header
// the message requires. Today, only the header related to paged message is
// supported and, for that, we send a message to the extension having this
// "custom header" as payload before sending the actual message with the
// actual payload.
func (c Connection) sendMessage(msg Message) error {
	if msg.length > payloadMaxLen {
		// Create new message to indicate the number of pages:
		// """
		// --tbed-hdr
		// Pages: %d
		// """
		// we might make more use of that in the future.
		header := "--tbed-hdr\n"
		hdrPages := fmt.Sprintf("%sPages: %d", header, msg.pages)
		encPages, err := json.Marshal(hdrPages)
		if err != nil {
			return err
		}

		pagesMsg := Message{}
		pagesMsg.payload = string(encPages)
		if err := c.send(pagesMsg); err != nil {
			return err
		}
	}

	if err := c.send(msg); err != nil {
		return err
	}

	return nil
}
