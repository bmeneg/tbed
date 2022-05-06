package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
)

type Editor struct {
	cmd string
}

// run executes the command considering arguments in the default shell format.
// FIXME: I'm not quite not sure how it works on Windows yet.
func (ed Editor) run(path string) error {
	dbg(fmt.Sprintf("ed.cmd: %s. path: %s", ed.cmd, path))
	// Split editor command using shell rules for quoting and commenting
	parts, err := shlex.Split(ed.cmd)
	if err != nil {
		return err
	}

	var args []string
	name := parts[0]
	if len(parts) > 0 {
		for _, arg := range parts[1:] {
			arg = strings.Replace(arg, "'", "\"", -1)
			args = append(args, arg)
		}
	}
	args = append(args, path)

	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	dbg(fmt.Sprintf("running editor %s", ed.cmd))
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// externalEdit creates a temp file for storing the text to be edited, open
// it with the editor and, once the editor is closed, the file is read and
// deleted. The content is then returned for further processing.
func (ed Editor) edit(text string) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "tbed-")
	if err != nil {
		return "", err
	}
	defer file.Close()
	defer os.Remove(file.Name())
	dbg(fmt.Sprintf("edit: file %s created", file.Name()))

	if _, err := file.WriteString(text); err != nil {
		return "", err
	}

	if err := ed.run(file.Name()); err != nil {
		return "", err
	}

	var content []byte
	file, err = os.Open(file.Name())
	if err != nil {
		return "", err
	}
	content, err = ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// initEditor instantiate an Editor object gathering the command to be executed
// in the message comming from the extension.
func initEditor(msg Message) (Editor, error) {
	editor := Editor{}

	// Editor command message format:
	// """"
	// --tbed-hdr
	// Command: <cmd>
	// """"
	dbg(fmt.Sprintf("editor message payload:\n%s", msg.payload))

	hdrLine := make([]byte, len(tbedHeader))
	reader := strings.NewReader(msg.payload[0])
	if _, err := io.ReadFull(reader, hdrLine); err != nil {
		return editor, err
	}

	if string(hdrLine) == tbedHeader {
		hdrLine = make([]byte, reader.Len())
		if _, err := io.ReadFull(reader, hdrLine); err != nil {
			return editor, err
		}

		if strings.Contains(string(hdrLine), "Command") {
			editor.cmd = strings.SplitN(string(hdrLine), " ", 2)[1]
		}
	}

	return editor, nil
}
