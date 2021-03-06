// Copyright 2020 Thomas.Hoehenleitner [at] seerose.net
// All rights reserved.
// Use of this source code is governed by a
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rokath/trice/pkg/com"
	"github.com/rokath/trice/pkg/emit"
	"github.com/rokath/trice/pkg/id"
	"github.com/tarm/serial"
)

// HandleArgs evaluates the arguments slice of strings und uses wd as working directory
func HandleArgs(wd string, args []string) error {
	list := make(id.List, 0, 65536) // for 16 bit IDs enough
	pList := &list

	uCmd := flag.NewFlagSet("update", flag.ExitOnError)                             // subcommand
	pSrcU := uCmd.String("src", wd, "source dir or file (optional, default is ./)") // flag
	pDryR := uCmd.Bool("dry-run", false, "no changes are applied (optional)")       // flag
	pLU := uCmd.String("list", "til.json", "trice ID list path (optional)")         // flag

	lCmd := flag.NewFlagSet("log", flag.ExitOnError)                                // subcommand
	pPort := lCmd.String("port", "", "subcommand (required, try COMscan)")          // flag
	pBaud := lCmd.Int("baud", 38400, "COM baudrate (optional, default is 38400")    // flag
	pL := lCmd.String("list", "til.json", "trice ID list path (optional)")          // flag
	pCol := lCmd.String("color", "default", "color set (optional), off, alternate") // flag

	cCmd := flag.NewFlagSet("check", flag.ExitOnError)                              // subcommand
	pSet := cCmd.String("dataset", "position", "parameters (optional), negative")   // flag
	pC := cCmd.String("list", "til.json", "trice ID list path (optional)")          // flag
	pPal := cCmd.String("color", "default", "color set (optional), off, alternate") // flag

	zCmd := flag.NewFlagSet("zeroSourceTreeIds", flag.ContinueOnError)                  // subcommand (during development only)
	pSrcZ := zCmd.String("src", "", "zero all Id(n) inside source tree dir (required)") // flag
	pRunZ := zCmd.Bool("dry-run", false, "no changes are applied (optional)")           // flag

	hCmd := flag.NewFlagSet("help", flag.ContinueOnError) // subcommand

	vCmd := flag.NewFlagSet("version", flag.ContinueOnError) // subcommand

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		return errors.New("no args, try: 'trice help'")
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	// FlagSet.Parse() requires a set of arguments to parse as input
	// os.Args[2:] will be all arguments starting after the subcommand at os.Args[1]
	subCmd := args[1]
	subArgs := args[2:]
	var err error
	switch subCmd { // Check which subcommand is invoked.
	case "h":
		fallthrough
	case "help":
		err = hCmd.Parse(subArgs)
	case "v":
		fallthrough
	case "ver":
		fallthrough
	case "version":
		err = vCmd.Parse(subArgs)
	case "u":
		fallthrough
	case "upd":
		fallthrough
	case "update":
		err = uCmd.Parse(subArgs)
	case "check":
		err = cCmd.Parse(subArgs)
	case "l":
		fallthrough
	case "log":
		err = lCmd.Parse(subArgs)
	case "zeroSourceTreeIds":
		err = zCmd.Parse(subArgs)
	default:
		fmt.Println("try: 'trice help|h'")
		return nil
	}
	if nil != err {
		return fmt.Errorf("failed to parse %s: %v", subArgs, err)
	}
	// Check which subcommand was Parsed using the FlagSet.Parsed() function. Handle each case accordingly.
	// FlagSet.Parse() will evaluate to false if no flags were parsed (i.e. the user did not provide any flags)
	if hCmd.Parsed() {
		return help(hCmd, uCmd, cCmd, lCmd, zCmd, vCmd)
	}
	if uCmd.Parsed() {
		lU, err := filepath.Abs(*pLU)
		if nil != err {
			return fmt.Errorf("failed to parse %s: %v", *pLU, err)
		}
		srcU, err := filepath.Abs(*pSrcU)
		if nil != err {
			return fmt.Errorf("failed to parse %s: %v", *pSrcU, err)
		}
		return update(*pDryR, srcU, lU, pList)
	}
	if cCmd.Parsed() {
		return checkList(*pC, *pSet, pList, *pPal)
	}
	if lCmd.Parsed() {
		return logTraces(lCmd, *pPort, *pBaud, *pL, pList, *pCol)
	}
	if zCmd.Parsed() {
		return zeroIds(*pRunZ, *pSrcZ, zCmd)
	}
	if vCmd.Parsed() {
		return ver()
	}
	return nil
}

func ver() error {
	if "" != version {
		fmt.Printf("version=%v, commit=%v, built at %v", version, commit, date)
		return nil
	}
	fmt.Printf("version=devel, commit=unknown, built after 2020-02-10-1800")
	return errors.New("No goreleaser generated executable")
}

func help(hCmd *flag.FlagSet,
	uCmd *flag.FlagSet,
	cCmd *flag.FlagSet,
	lCmd *flag.FlagSet,
	zCmd *flag.FlagSet,
	vCmd *flag.FlagSet) error {
	fmt.Println("syntax: 'trice subcommand' [params]")
	fmt.Println("subcommand 'help', 'h'")
	hCmd.PrintDefaults()
	fmt.Println("subcommand 'update', 'upd', 'u'")
	uCmd.PrintDefaults()
	fmt.Println("subcommand 'check'")
	cCmd.PrintDefaults()
	fmt.Println("subcommand 'log', 'l'")
	lCmd.PrintDefaults()
	fmt.Println("subcommand 'zeroSourceTreeIds' (avoid using this subcommand normally)")
	zCmd.PrintDefaults()
	fmt.Println("subcommand 'version', 'ver'. 'v'")
	vCmd.PrintDefaults()
	fmt.Println("examples:")
	fmt.Println("    'trice update [-dir sourcerootdir]', default sourcerootdir is ./")
	fmt.Println("    'trice log [-port COMn] [-baud m]', default port is COMscan, default m is 38400, fixed to 8N1")
	fmt.Println("    'trice zeroSourceTreeIds -dir sourcerootdir]'")
	fmt.Println("    'trice version'")
	return ver()
}

// parse source tree, update IDs and is list
func update(dryRun bool, dir, fn string, p *id.List) error {
	err := p.Update(dir, fn, !dryRun)
	if nil != err {
		return fmt.Errorf("failed update on %s with %s: %v", dir, fn, err)
	}
	fmt.Println(len(*p), "ID's in list", fn)
	return nil
}

// log the id list with dataset
func checkList(fn, dataset string, p *id.List, palette string) error {
	err := p.Read(fn)
	if nil != err {
		fmt.Println("ID list " + fn + " not found, exit")
		return nil
	}
	emit.Check(*p, dataset, palette)
	return nil
}

// connect to port and display traces
func logTraces(cmd *flag.FlagSet, port string, baud int, fn string, p *id.List, palette string) error {
	if "" == port {
		cmd.PrintDefaults()
		return nil
	}
	if strings.Contains(port, "COM") { // true
		var pConfig = &serial.Config{
			Name:        port,
			Baud:        baud,
			ReadTimeout: 1,
			Size:        8,
		}
		if port == "COMscan" {
			com.FindSerialPorts(pConfig)
			return nil
		}
		stream, err := serial.OpenPort(pConfig)
		if err != nil {
			fmt.Println(pConfig.Name, "not found")
			fmt.Println("try -port COMscan")
			return err
		}
		defer stream.Close()
		err = p.Read(fn)
		if nil != err {
			fmt.Println("ID list " + fn + " not found, exit")
			return nil
		}
		fmt.Println("using id list file", fn, "with", len(*p), "items")
		com.ReadEndless(stream, *p, palette)

		return nil
	}
	msg := "cannot handle -port " + port
	return errors.New(msg)
}

// replace all ID's in sourc tree with 0
func zeroIds(dryRun bool, SrcZ string, cmd *flag.FlagSet) error {
	if SrcZ == "" {
		cmd.PrintDefaults()
		return errors.New("no source tree root specified")
	}
	id.ZeroSourceTreeIds(SrcZ, !dryRun)
	return nil
}
