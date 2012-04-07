package main

import (
	"github.com/kballard/dcpu16/dcpu"
	"github.com/kballard/dcpu16/dcpu/core"
	"fmt"
	"github.com/nsf/termbox-go"
	"io/ioutil"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s program\n", os.Args[0])
		os.Exit(2)
	}
	program := os.Args[1]
	data, err := ioutil.ReadFile(program)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Interpret the file as Words
	words := make([]core.Word, len(data)/2)
	for i := 0; i < len(data)/2; i++ {
		w := core.Word(data[i*2])<<8 + core.Word(data[i*2+1])
		words[i] = w
	}

	// Set up a machine
	machine := new(dcpu.Machine)
	if err := machine.State.LoadProgram(words, 0); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := machine.Start(dcpu.DefaultClockRate); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// now wait for the q key
	for {
		evt := termbox.PollEvent()
		if evt.Type == termbox.EventKey {
			if evt.Key == termbox.KeyCtrlC || (evt.Mod == 0 && evt.Ch == 'q') {
				if err := machine.Stop(); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				break
			}
		}
	}
}
