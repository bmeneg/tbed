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
	"strconv"
)

var externDebug string = "true"
var externLogFilename string = "tbed.log"

// LogOptions is just a helper to store logging specificities.
type LogOptions struct {
	fn    string
	debug bool
}

var logOpts LogOptions

// initLogger set some settings for logging data into a file instead of the
// default stdout/stderr, which are already being used for in/out of the
// MailExtension protocol.
func initLogger() error {
	debug, err := strconv.ParseBool(externDebug)
	if err != nil {
		return err
	}
	logOpts = LogOptions{
		fn:    externLogFilename,
		debug: debug,
	}

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

	// First, read the message with the editor cmd to be executed.
	editorMsg, err := extConn.readMessage()
	if err != nil {
		log.Fatal(err)
	}
	editor, err := initEditor(*editorMsg)
	if err != nil {
		log.Fatal(err)
	}

	// Second, read the message to be edited.
	textMsg, err := extConn.readMessage()
	if err != nil {
		log.Fatal(err)
	}

	// Third, update message payload with the edited version.
	text, err := editor.edit(textMsg.payload[0])
	if err != nil {
		log.Fatal(err)
	}

	retMsg := &Message{}
	if err = retMsg.setPayload(text); err != nil {
		log.Fatal(err)
	}

	// Finally, send the updated message back to the extension.
	if err = extConn.sendMessage(*retMsg); err != nil {
		log.Fatal(err)
	}
}
