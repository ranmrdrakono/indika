package indika

import (
	"debug/elf"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/go-errors/errors"
	be "github.com/ranmrdrakono/indika/blanket_emulator"
	ds "github.com/ranmrdrakono/indika/data_structures"
	"github.com/ranmrdrakono/indika/disassemble"
	loader "github.com/ranmrdrakono/indika/loader/elf"
	uc "github.com/unicorn-engine/unicorn/bindings/go/unicorn"
	"io"
	"os"
	//	"reflect"
	"strings"
	"testing"
)

func init() {
	//log.SetLevel(log.DebugLevel)
	log.SetLevel(log.ErrorLevel)
}

func find_mapping_for(maps map[ds.Range]*ds.MappedRegion, needle ds.Range) *ds.MappedRegion {
	for rng, mapping := range maps {
		if rng.IntersectsRange(needle) {
			return mapping
		}
	}
	return nil
}
func filter_empty_bbs(bbs map[ds.Range]bool) map[ds.Range]bool {
	res := make(map[ds.Range]bool)
	for rng, _ := range bbs {
		if !rng.IsEmpty() {
			res[rng] = true
		}
	}
	return res
}

func extract_bbs(maps map[ds.Range]*ds.MappedRegion, rng ds.Range) map[ds.Range]bool {
	maped := find_mapping_for(maps, rng)
	if maped == nil {
		return nil
	}
	blocks := disassemble.GetBasicBlocks(maped.Range.From, maped.Data, rng)
	return filter_empty_bbs(blocks)
}

func mapKeysRangeToStarts(mem map[ds.Range]*ds.MappedRegion) map[uint64][]byte {
	res := make(map[uint64][]byte)
	for key, val := range mem {
		res[key.From] = (*val).Data
	}
	return res
}

func MakeBlanketEmulator(mem map[ds.Range]*ds.MappedRegion) *be.Emulator {
	ev := be.NewEventsToMinHash()
	config := be.Config{
		MaxTraceInstructionCount: 100,
		MaxTraceTime:             0,
		MaxTracePages:            100,
		Arch:                     uc.ARCH_X86,
		Mode:                     uc.MODE_64,
		EventHandler:             ev,
	}
	mem_starts := mapKeysRangeToStarts(mem)
	em, err := be.NewEmulator(mem_starts, config)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Fatal("Error creating Emulator")
	}
	return em
}

func ioReader(file string) io.ReaderAt {
	r, err := os.Open(file)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Fatal("Error creating File Reader")
	}
	return r
}

func wrap(err error) *errors.Error {
	if err != nil {
		return errors.Wrap(err, 1)
	}
	return nil
}

func check(err *errors.Error) {
	if err != nil && err.Err != nil {
		log.WithFields(log.Fields{"error": err, "stack": err.ErrorStack()}).Fatal("Error creating Elf Parser")
	}
}

func TestRun(t *testing.T) {
	file := "samples/simple/O0/strings"
	f := ioReader(file)
	_elf, err := elf.NewFile(f)
	check(wrap(err))
	maps := loader.GetSegments(_elf)
	symbols := loader.GetSymbols(_elf)
	fmt.Println("done loading")
	emulator := MakeBlanketEmulator(maps)
	fmt.Println("done making blanket emulator")

	for rng, symb := range symbols {
		if symb.Type == ds.FUNC && strings.Contains(symb.Name, "str") {
			bbs := extract_bbs(maps, rng)
			if len(bbs) == 0 {
				continue
			}
			fmt.Printf("found function %v\n", symb.Name)
			fmt.Printf("running for %v \n", bbs)
			err := emulator.FullBlanket(bbs)
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Fatal("Error running Blanket")
			}
			ev := emulator.Config.EventHandler.(*be.EventsToMinHash)
			fmt.Println("hash %v", ev.GetHash(60))
			//			fmt.Println("events %v", ev.Inspect())
		}
	}
}
