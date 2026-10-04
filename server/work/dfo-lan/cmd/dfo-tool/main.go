// dfo-tool is the single entry point for offline diagnostics and maintenance.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"dfolan/internal/toolcmd/audit36"
	"dfolan/internal/toolcmd/charactercheck"
	"dfolan/internal/toolcmd/dbq"
	"dfolan/internal/toolcmd/dungeonimport"
	"dfolan/internal/toolcmd/dungeonscenesaudit"
	"dfolan/internal/toolcmd/equipfields"
	"dfolan/internal/toolcmd/equipmentfull"
	"dfolan/internal/toolcmd/framedump"
	"dfolan/internal/toolcmd/initialrepair"
	"dfolan/internal/toolcmd/loginchannel"
	"dfolan/internal/toolcmd/npcpresenceaudit"
	"dfolan/internal/toolcmd/protocolfixture"
	"dfolan/internal/toolcmd/pvfaudit"
	"dfolan/internal/toolcmd/pvfinspect"
	"dfolan/internal/toolcmd/questchain"
	"dfolan/internal/toolcmd/questrepair"
	"dfolan/internal/toolcmd/shieldaudit"
	"dfolan/internal/toolcmd/skillaudit"
	"dfolan/internal/toolcmd/sqliteconvert"
	"dfolan/internal/toolcmd/storagecheck"
	"dfolan/internal/toolcmd/townprobe"
)

type command struct {
	name, group, usage string
	flags              bool
	minimumArgs        int
	run                func()
}

// Keep commands sorted by name. Implementations register flags only when run.
var commands = []command{
	{"audit36", "audit", "[options]", true, 0, audit36.Run},
	{"charactercheck", "maintenance", "(temporary-schema storage regression; no options)", false, 0, charactercheck.Run},
	{"dbq", "maintenance", "[options]", true, 0, dbq.Run},
	{"dungeonimport", "catalog", "[options]", true, 0, dungeonimport.Run},
	{"dungeonscenesaudit", "audit", "[options]", true, 0, dungeonscenesaudit.Run},
	{"equipfields", "catalog", "[options]", true, 0, equipfields.Run},
	{"equipmentfull", "catalog", "[options]", true, 0, equipmentfull.Run},
	{"framedump", "protocol", "<label> <key.bin> <stream.bin> <all|id|offset:size> [header]", false, 4, framedump.Run},
	{"initialrepair", "maintenance", "[options]", true, 0, initialrepair.Run},
	{"loginchannel", "protocol", "<input.bin> <output.bin> <channel-type>", false, 3, loginchannel.Run},
	{"npcpresenceaudit", "audit", "[options]", true, 0, npcpresenceaudit.Run},
	{"protocolfixture", "protocol", "<output.bin> [login|characters|name|characters-row]", false, 1, protocolfixture.Run},
	{"pvfaudit", "pvf", "[options]", true, 0, pvfaudit.Run},
	{"pvfinspect", "pvf", "[options]", true, 0, pvfinspect.Run},
	{"questchain", "catalog", "[options]", true, 0, questchain.Run},
	{"questrepair", "maintenance", "[options]", true, 0, questrepair.Run},
	{"shieldaudit", "audit", "[options]", true, 0, shieldaudit.Run},
	{"skillaudit", "audit", "[options]", true, 0, skillaudit.Run},
	{"sqliteconvert", "maintenance", "[options]", true, 0, sqliteconvert.Run},
	{"storagecheck", "maintenance", "[options]", true, 0, storagecheck.Run},
	{"townprobe", "catalog", "[options]", true, 0, townprobe.Run},
}

func main() { os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr)) }

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "Usage: dfo-tool <command> [arguments]")
	fmt.Fprintln(out, "Run from server/work/dfo-lan; relative input/output paths use the working directory.")
	fmt.Fprintln(out, "Use dfo-tool <command> -h for command help. Maintenance/patch commands retain their existing effects.")
	for _, group := range []string{"pvf", "catalog", "audit", "maintenance", "protocol"} {
		fmt.Fprintf(out, "\n%s:\n", group)
		for _, c := range commands {
			if c.group == group {
				fmt.Fprintf(out, "  %-26s %s\n", c.name, c.usage)
			}
		}
	}
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printHelp(stdout)
		return 0
	}
	for _, c := range commands {
		if c.name != args[0] {
			continue
		}
		tail := args[1:]
		if !c.flags {
			if len(tail) == 1 && (tail[0] == "-h" || tail[0] == "--help") {
				fmt.Fprintf(stdout, "Usage: dfo-tool %s %s\n", c.name, c.usage)
				return 0
			}
			if len(tail) < c.minimumArgs {
				fmt.Fprintf(stderr, "Usage: dfo-tool %s %s\n", c.name, c.usage)
				return 2
			}
		}
		// Legacy tools use either flag.CommandLine or os.Args. Supply precisely
		// the same argument positions as their former standalone executable.
		originalArgs, originalFlags := os.Args, flag.CommandLine
		defer func() { os.Args, flag.CommandLine = originalArgs, originalFlags }()
		os.Args = append([]string{"dfo-tool " + c.name}, tail...)
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		flag.CommandLine.SetOutput(stderr)
		c.run()
		return 0
	}
	fmt.Fprintf(stderr, "Unknown command %q; use dfo-tool -h.\n", args[0])
	return 2
}
