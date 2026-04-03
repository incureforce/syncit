package cli

import (
	"embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed version.tag
var versionFS embed.FS

func init() {
	rootCmd.AddCommand(cmdVersion())
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			version := currentBuiltVersion()

			fmt.Printf("%s\n", version)
			return nil
		},
	}
}

func currentBuiltVersion() string {
	version := "unknown"

	if data, err := versionFS.ReadFile("version.tag"); err == nil {
		bomLen, _ := detectBOM(data)
		content := strings.TrimSpace(string(data[bomLen:]))

		if content != "" {
			version = content
		}
	}

	return version
}

func detectBOM(data []byte) (bomLen int, encoding string) {
	switch {
	case len(data) >= 3 &&
		data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF:
		return 3, "UTF-8"

	case len(data) >= 2 &&
		data[0] == 0xFF && data[1] == 0xFE:
		return 2, "UTF-16 LE"

	case len(data) >= 2 &&
		data[0] == 0xFE && data[1] == 0xFF:
		return 2, "UTF-16 BE"

	case len(data) >= 4 &&
		data[0] == 0xFF && data[1] == 0xFE &&
		data[2] == 0x00 && data[3] == 0x00:
		return 4, "UTF-32 LE"

	case len(data) >= 4 &&
		data[0] == 0x00 && data[1] == 0x00 &&
		data[2] == 0xFE && data[3] == 0xFF:
		return 4, "UTF-32 BE"
	}

	return 0, "Unknown/None"
}
