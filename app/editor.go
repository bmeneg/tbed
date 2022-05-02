package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
)

type Editor struct {
	execCall string
	filePath string
}

// run executes the command considering arguments in the default shell format.
// FIXME: I'm not quite not sure how it works on Windows yet.
func (ed *Editor) run() error {
	// Split editor command using shell rules for quoting and commenting
	parts, err := shlex.Split(ed.execCall)
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
	args = append(args, ed.filePath)

	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	dbg(fmt.Sprintf("running editor %s", ed.execCall))
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// externalEdit creates a temp file for storing the text to be edited, open
// it with the editor and, once the editor is closed, the file is read and
// deleted. The content is then returned for further processing.
func externalEdit(text string) (string, error) {
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

	editor := &Editor{}
	// FIXME: remove hard-coded editor path/command.
	editor.execCall = "nvim-qt.exe"
	editor.filePath = file.Name()
	if err := editor.run(); err != nil {
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
