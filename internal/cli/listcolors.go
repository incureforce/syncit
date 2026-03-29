package cli

import "github.com/fatih/color"

// Helpers for informational `ls` output (fatih/color respects NO_COLOR and tty).
var (
	listKey   = color.New(color.FgCyan).SprintFunc()
	listMuted = color.New(color.FgHiBlack).SprintFunc()
	listWarn  = color.New(color.FgYellow).SprintFunc()
	listTag   = color.New(color.FgMagenta).SprintFunc()
	listEmpty = color.New(color.FgHiBlack).SprintFunc()

	syncGood = color.New(color.FgGreen).SprintFunc()
	syncBad  = color.New(color.FgRed).SprintFunc()
)
