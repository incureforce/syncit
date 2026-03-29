package cli

import (
	"github.com/fatih/color"

	"go-syncit/internal/client/db"
)

type trackedStatus struct {
	Label string
	Paint func(a ...interface{}) string
}

func trackedLineStatus(tf clientdb.TrackedFile) trackedStatus {
	switch {
	case tf.Conflict:
		return trackedStatus{"CONFLICT", color.New(color.FgRed, color.Bold).SprintFunc()}
	case tf.FileHash != tf.RemoteHash:
		return trackedStatus{"pending", color.New(color.FgYellow).SprintFunc()}
	default:
		return trackedStatus{"synced", color.New(color.FgGreen).SprintFunc()}
	}
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12] + "…"
}
