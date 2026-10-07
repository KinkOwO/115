package launcher

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// errNoPVFSupport is returned when a PVF-direct run selects a gateway that does not
// advertise PVF support. Starting anyway would produce a server that silently reads a
// different content source, so the Python refuses and so does this.
var errNoPVFSupport = errors.New(
	"the selected gateway does not advertise PVF direct support; rebuild it or select JSON mode explicitly")

// flagLine matches one flag in a Go program's usage output.
var flagLine = regexp.MustCompile(`^\s+-([A-Za-z0-9._-]+)`)

// ExeFlags lists the flags a binary recognises by running `<exe> -h` and parsing its usage.
//
// It returns nil when the probe cannot run or the output names nothing, and callers must
// treat nil as "unknown" rather than "supports nothing": dropping flags from a binary we
// merely failed to interrogate is how a working build gets broken.
func ExeFlags(ctx context.Context, binary string) map[string]bool {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, binary, "-h")
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil
	}
	names := map[string]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		if match := flagLine.FindStringSubmatch(strings.TrimRight(line, "\r")); match != nil {
			names[match[1]] = true
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// PruneCommand drops flags the target binary does not define, together with their values,
// keeping everything else in order.
//
// supported == nil means the probe failed and the command is returned untouched. This is
// deliberate: the historical failure this guards against was a build that aborted at start
// with "exit status 1" because flags for a newer profile were passed to an older binary,
// and the fix must not overshoot into stripping valid arguments.
//
// The command shape is assumed to be [-flag value -flag -flag value ...], so a value is
// recognised as "the next token does not start with -".
func PruneCommand(command []string, supported map[string]bool, pvfCatalogs bool) ([]string, []string, error) {
	if len(command) == 0 {
		return command, nil, nil
	}
	if pvfCatalogs && (supported == nil || !supported["pvf-catalogs"]) {
		return nil, nil, errNoPVFSupport
	}
	if supported == nil {
		return command, nil, nil
	}

	kept := []string{command[0]}
	var dropped []string
	for index := 1; index < len(command); {
		token := command[index]
		if !strings.HasPrefix(token, "-") || supported[strings.TrimPrefix(token, "-")] {
			kept = append(kept, token)
			index++
			continue
		}
		dropped = append(dropped, token)
		index++
		if index < len(command) && !strings.HasPrefix(command[index], "-") {
			dropped = append(dropped, command[index])
			index++
		}
	}
	return kept, dropped, nil
}

// PruneUnsupported is the probe-time form: it interrogates the binary and then prunes.
func PruneUnsupported(ctx context.Context, command []string, pvfCatalogs bool) ([]string, []string, error) {
	if len(command) == 0 {
		return command, nil, nil
	}
	return PruneCommand(command, ExeFlags(ctx, command[0]), pvfCatalogs)
}

// SetOptionValue replaces an existing option's value, or appends the pair when the flag is
// new to this profile. Mirrors set_option_value in the probe.
func SetOptionValue(command []string, option, value string) []string {
	for index, token := range command {
		if token == option {
			if index+1 < len(command) {
				command[index+1] = value
				return command
			}
			return append(command, value)
		}
	}
	return append(command, option, value)
}

// DropWarning renders the probe's warning for the flags it had to remove, so a degraded
// start is visible rather than silent.
func DropWarning(binary string, dropped []string) string {
	if len(dropped) == 0 {
		return ""
	}
	return fmt.Sprintf("WARNING: %s does not define %s; dropped them so this build can still start.",
		binary, strings.Join(dropped, ", "))
}
