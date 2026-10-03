// dfo-tool is the single entry point for offline diagnostics and maintenance.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"dfolan/internal/toolcmd/apocalypseimport"
	"dfolan/internal/toolcmd/attunementimport"
	"dfolan/internal/toolcmd/audit36"
	"dfolan/internal/toolcmd/avatarrestorecheck"
	"dfolan/internal/toolcmd/boosterexport"
	"dfolan/internal/toolcmd/catalogimport"
	"dfolan/internal/toolcmd/charactercheck"
	"dfolan/internal/toolcmd/dbq"
	"dfolan/internal/toolcmd/dump342"
	"dfolan/internal/toolcmd/dump781x"
	"dfolan/internal/toolcmd/dungeonimport"
	"dfolan/internal/toolcmd/dungeonscenesaudit"
	"dfolan/internal/toolcmd/equipfields"
	"dfolan/internal/toolcmd/equipmentaudit"
	"dfolan/internal/toolcmd/equipmentfull"
	"dfolan/internal/toolcmd/equipmentfullimport"
	"dfolan/internal/toolcmd/equipmentjournalimport"
	"dfolan/internal/toolcmd/equipmentwearimport"
	"dfolan/internal/toolcmd/framedump"
	"dfolan/internal/toolcmd/hellpartyimport"
	"dfolan/internal/toolcmd/initialrepair"
	"dfolan/internal/toolcmd/itemperiodimport"
	"dfolan/internal/toolcmd/itemshopimport"
	"dfolan/internal/toolcmd/legionimport"
	"dfolan/internal/toolcmd/loginchannel"
	"dfolan/internal/toolcmd/lootimport"
	"dfolan/internal/toolcmd/lotterycatalog"
	"dfolan/internal/toolcmd/npcpresenceaudit"
	"dfolan/internal/toolcmd/npcteleportimport"
	"dfolan/internal/toolcmd/oathgradeimport"
	"dfolan/internal/toolcmd/odysseyaudit"
	"dfolan/internal/toolcmd/odysseychapterimport"
	"dfolan/internal/toolcmd/odysseygrowthaudit"
	"dfolan/internal/toolcmd/progressionimport"
	"dfolan/internal/toolcmd/protocolfixture"
	"dfolan/internal/toolcmd/pvfaudit"
	"dfolan/internal/toolcmd/pvfinspect"
	"dfolan/internal/toolcmd/pvflist"
	"dfolan/internal/toolcmd/pvfpatch"
	"dfolan/internal/toolcmd/questaudit"
	"dfolan/internal/toolcmd/questcatalog"
	"dfolan/internal/toolcmd/questchain"
	"dfolan/internal/toolcmd/questequipmentimport"
	"dfolan/internal/toolcmd/questrepair"
	"dfolan/internal/toolcmd/randomoptionimport"
	"dfolan/internal/toolcmd/selectionboximport"
	"dfolan/internal/toolcmd/shieldaudit"
	"dfolan/internal/toolcmd/shopaudit"
	"dfolan/internal/toolcmd/shopimport"
	"dfolan/internal/toolcmd/shopprices"
	"dfolan/internal/toolcmd/skillaudit"
	"dfolan/internal/toolcmd/skinstorageimport"
	"dfolan/internal/toolcmd/storagecheck"
	"dfolan/internal/toolcmd/towerdazzlementimport"
	"dfolan/internal/toolcmd/towergriefimport"
	"dfolan/internal/toolcmd/towncatalog"
	"dfolan/internal/toolcmd/townprobe"
	"dfolan/internal/toolcmd/tutorialimport"
	"dfolan/internal/toolcmd/worldcatalog"
)

type command struct {
	name, group, usage string
	flags              bool
	minimumArgs        int
	run                func()
}

// Keep commands sorted by name. Implementations register flags only when run.
var commands = []command{
	{"apocalypseimport", "catalog", "[options]", true, 0, apocalypseimport.Run},
	{"attunementimport", "catalog", "[options]", true, 0, attunementimport.Run},
	{"audit36", "audit", "[options]", true, 0, audit36.Run},
	{"avatarrestorecheck", "maintenance", "(historical fixed-input diagnostic)", false, 0, avatarrestorecheck.Run},
	{"boosterexport", "catalog", "[options]", true, 0, boosterexport.Run},
	{"catalogimport", "catalog", "[options]", true, 0, catalogimport.Run},
	{"charactercheck", "maintenance", "(temporary-schema storage regression; no options)", false, 0, charactercheck.Run},
	{"dbq", "maintenance", "[options]", true, 0, dbq.Run},
	{"dump342", "protocol", "<key.bin> <stream.bin>", false, 2, dump342.Run},
	{"dump781x", "protocol", "(historical fixed-input diagnostic)", false, 0, dump781x.Run},
	{"dungeonimport", "catalog", "[options]", true, 0, dungeonimport.Run},
	{"dungeonscenesaudit", "audit", "[options]", true, 0, dungeonscenesaudit.Run},
	{"equipfields", "catalog", "[options]", true, 0, equipfields.Run},
	{"equipmentaudit", "audit", "[options]", true, 0, equipmentaudit.Run},
	{"equipmentfull", "catalog", "[options]", true, 0, equipmentfull.Run},
	{"equipmentfullimport", "catalog", "[options]", true, 0, equipmentfullimport.Run},
	{"equipmentjournalimport", "catalog", "[options]", true, 0, equipmentjournalimport.Run},
	{"equipmentwearimport", "catalog", "[options]", true, 0, equipmentwearimport.Run},
	{"framedump", "protocol", "<label> <key.bin> <stream.bin> <all|id|offset:size> [header]", false, 4, framedump.Run},
	{"hellpartyimport", "catalog", "[options]", true, 0, hellpartyimport.Run},
	{"initialrepair", "maintenance", "[options]", true, 0, initialrepair.Run},
	{"itemperiodimport", "catalog", "[options]", true, 0, itemperiodimport.Run},
	{"itemshopimport", "catalog", "[options]", true, 0, itemshopimport.Run},
	{"legionimport", "catalog", "[options]", true, 0, legionimport.Run},
	{"loginchannel", "protocol", "<input.bin> <output.bin> <channel-type>", false, 3, loginchannel.Run},
	{"lootimport", "catalog", "[options]", true, 0, lootimport.Run},
	{"lotterycatalog", "catalog", "[options]", true, 0, lotterycatalog.Run},
	{"npcpresenceaudit", "audit", "[options]", true, 0, npcpresenceaudit.Run},
	{"npcteleportimport", "catalog", "[options]", true, 0, npcteleportimport.Run},
	{"oathgradeimport", "catalog", "[options]", true, 0, oathgradeimport.Run},
	{"odysseyaudit", "audit", "(historical fixed-input diagnostic)", false, 0, odysseyaudit.Run},
	{"odysseychapterimport", "catalog", "[options]", true, 0, odysseychapterimport.Run},
	{"odysseygrowthaudit", "audit", "[options]", true, 0, odysseygrowthaudit.Run},
	{"progressionimport", "catalog", "[options]", true, 0, progressionimport.Run},
	{"protocolfixture", "protocol", "<output.bin> [login|characters|name|characters-row]", false, 1, protocolfixture.Run},
	{"pvfaudit", "pvf", "[options]", true, 0, pvfaudit.Run},
	{"pvfinspect", "pvf", "[options]", true, 0, pvfinspect.Run},
	{"pvflist", "pvf", "[options]", true, 0, pvflist.Run},
	{"pvfpatch", "pvf", "[options]", true, 0, pvfpatch.Run},
	{"questaudit", "audit", "(historical fixed-input diagnostic)", false, 0, questaudit.Run},
	{"questcatalog", "catalog", "[options]", true, 0, questcatalog.Run},
	{"questchain", "catalog", "[options]", true, 0, questchain.Run},
	{"questequipmentimport", "catalog", "[options]", true, 0, questequipmentimport.Run},
	{"questrepair", "maintenance", "[options]", true, 0, questrepair.Run},
	{"randomoptionimport", "catalog", "[options]", true, 0, randomoptionimport.Run},
	{"selectionboximport", "catalog", "[options]", true, 0, selectionboximport.Run},
	{"shieldaudit", "audit", "[options]", true, 0, shieldaudit.Run},
	{"shopaudit", "audit", "[options]", true, 0, shopaudit.Run},
	{"shopimport", "catalog", "[options]", true, 0, shopimport.Run},
	{"shopprices", "catalog", "[options]", true, 0, shopprices.Run},
	{"skillaudit", "audit", "[options]", true, 0, skillaudit.Run},
	{"skinstorageimport", "catalog", "[options]", true, 0, skinstorageimport.Run},
	{"storagecheck", "maintenance", "[options]", true, 0, storagecheck.Run},
	{"towerdazzlementimport", "catalog", "[options]", true, 0, towerdazzlementimport.Run},
	{"towergriefimport", "catalog", "[options]", true, 0, towergriefimport.Run},
	{"towncatalog", "catalog", "[options]", true, 0, towncatalog.Run},
	{"townprobe", "catalog", "[options]", true, 0, townprobe.Run},
	{"tutorialimport", "catalog", "[options]", true, 0, tutorialimport.Run},
	{"worldcatalog", "catalog", "[options]", true, 0, worldcatalog.Run},
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
