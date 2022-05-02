// This is the native application to talk with the TBEd ThunderBird extension.
//
// Author: Bruno Meneguele <bmeneguele@gmail.com>

// As explained in the ../extension/tbed.js file header, the communication
// between the extension and this app is done via the NativeMessaging protocol,
// which uses both stdin and stdout for messages exchange.
// With that, we're forced to send any logging to a specific file, since we
// can't force ThunderBird to handle app's mess.
//
// However, there still are cases where the application can "panic()" due to
// programmer's fault, and thus the backtrace is sent to ThunderBird's debug
// console.
//
// Note: this app only support one message at a time from the extension, up to
// 4GB, but we've implemented a multi-page response to the extension, up to
// 1MB per page.

package main

import (
	"log"
	"os"
)

// LogOptions is just a helper to store logging specificities.
type LogOptions struct {
	fn    string
	debug bool
}

var logOpts = LogOptions{fn: "tbed.log", debug: true}

// initLogger set some settings for logging data into a file instead of the
// default stdout/stderr, which are already being used for in/out of the
// MailExtension protocol.
func initLogger() error {
	fd, err := os.Create(logOpts.fn)
	if err != nil {
		return err
	}

	log.SetOutput(fd)
	return nil
}

// dbg is a wrapper around log.Print() when debug is enabled.
func dbg(msg string) {
	if logOpts.debug {
		log.Print("dbg: ", msg)
	}
}

func main() {
	if err := initLogger(); err != nil {
		// If local logger can't be set, send it to stdout and let the
		// extension log the error for us.
		log.SetOutput(os.Stdout)
		log.Fatalf("tbed app failed to create local logger: %s\n", err)
	}

	extConn := initConnection()
	msg, err := extConn.readMessage()
	if err != nil {
		log.Fatal(err)
	}

	// Update message payload with the edited version.
	text, err := externalEdit(msg.payload)
	if err != nil {
		log.Fatal(err)
	}
	msg.setPayload(text)
	if err = extConn.sendMessage(*msg); err != nil {
		log.Fatal(err)
	}
}
