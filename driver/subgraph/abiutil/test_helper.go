package abiutil

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"io"
	"os"
)

type Printer struct {
	io.Writer
}

func NewStdOutPrinter() Printer {
	return Printer{Writer: os.Stdout}
}

func (p Printer) Printf(format string, a ...any) {
	_, _ = fmt.Fprintf(p.Writer, format, a...)
}

func (p Printer) PrintMethod(md *abi.Method, title string) {
	p.Printf("=== METHOD:%s =================\n", title)
	p.Printf("Name: %s\n", md.Name)
	p.Printf("ID: %x %v\n", md.ID, md.ID)
	p.Printf("SigForID: %s\n", md.Sig)
	p.Printf("Sig: %s\n", GetMethodSig(md, true))
	for i, input := range md.Inputs {
		p.PrintArg(&input, fmt.Sprintf("Input[%d]", i))
	}
	for i, output := range md.Outputs {
		p.PrintArg(&output, fmt.Sprintf("Output[%d]", i))
	}
}

func (p Printer) PrintEvent(ev *abi.Event, title string) {
	p.Printf("=== EVENT:%s =================\n", title)
	p.Printf("Name: %s\n", ev.Name)
	p.Printf("RawName: %s\n", ev.RawName)
	p.Printf("ID: %x %v\n", ev.ID, ev.ID)
	p.Printf("SigForID: %s\n", ev.Sig)
	p.Printf("Sig: %s\n", GetEventSig(ev))
	for i, input := range ev.Inputs {
		p.PrintArg(&input, fmt.Sprintf("Input[%d]", i))
	}
}

func (p Printer) PrintArg(arg *abi.Argument, prefix string) {
	p.Printf("%s: %#v\n", prefix, arg)
	p.PrintType(&arg.Type, prefix+".Type")
}

func (p Printer) PrintType(typ *abi.Type, prefix string) {
	p.Printf("%s.stringKind: %s\n", prefix, typ.String())
	p.Printf("%s.T: %v\n", prefix, typ.T)
	p.Printf("%s.Size: %v\n", prefix, typ.Size)
	if typ.TupleRawName != "" {
		p.Printf("%s.TupleRawName: %v\n", prefix, typ.TupleRawName)
		p.Printf("%s.TupleRawNames: %v\n", prefix, typ.TupleRawNames)
		p.Printf("%s.TupleType: %v\n", prefix, typ.TupleType)
		for i, el := range typ.TupleElems {
			p.PrintType(el, fmt.Sprintf("%s.TupleElems[%d]", prefix, i))
		}
	}
	if typ.Elem != nil {
		p.PrintType(typ.Elem, prefix+".Elem")
	}
}
