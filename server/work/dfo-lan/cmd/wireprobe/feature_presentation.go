package main

import (
	"dfolan/internal/activity"
	"fmt"
	"strings"
)

// This label is presentation only. Claim identity, rewards and progression are
// still derived from the selected archive and never parsed back from the label.
func routineRewardLabel(rules activity.RoutineRules, title string, reward activity.Reward) string {
	name := rules.ItemNames[reward.Item]
	if name == "" {
		return title
	} // Synthetic tests may not bind an archive.
	return fmt.Sprintf("%s：%s×%d", title, name, reward.Count)
}

func membershipGradeLabel(name string) string {
	// CN source grade strings include the enclosing page heading. Preserve the
	// source's grade name while removing that redundant presentation prefix.
	if i := strings.IndexRune(name, '·'); i >= 0 {
		return name[i+len("·"):]
	}
	return name
}
