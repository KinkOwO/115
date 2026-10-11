package gamedata

import (
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"fmt"
	"path"
	"strings"
)

// AttendanceActivities reuses the selected archive. JSON exports are test
// fixtures only; the runtime never loads them or falls back to another source.
func (s *Source) AttendanceActivities(index catalog.ItemIndex) (*activity.Catalog, error) {
	if s == nil || s.archive == nil {
		return nil, fmt.Errorf("attendance activities require PVF")
	}
	c, err := activity.Load(s.archive)
	if err != nil {
		return nil, err
	}
	if err = s.bindAttendanceLogin(&c.Login, index); err != nil {
		return nil, err
	}
	for id := range c.RewardItems() {
		item, ok := index.Items[id]
		if !ok {
			return nil, fmt.Errorf("activity reward %d missing from selected item index", id)
		}
		if _, ok = s.archive.FindFile(item.Path); !ok {
			return nil, fmt.Errorf("activity reward %d missing script %s", id, item.Path)
		}
	}
	return c, nil
}

// AttendanceLogin prepares this campaign independently of the other event
// families, which have separate native contracts and import requirements.
func (s *Source) AttendanceLogin(index catalog.ItemIndex) (*activity.Login, error) {
	if s == nil || s.archive == nil {
		return nil, fmt.Errorf("attendance login requires PVF")
	}
	tokens, err := s.archive.Tokens(activity.LoginPath)
	if err != nil {
		return nil, err
	}
	c, err := activity.ParseLogin(tokens)
	if err != nil {
		return nil, err
	}
	if err = s.bindAttendanceLogin(&c, index); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Source) bindAttendanceLogin(c *activity.Login, index catalog.ItemIndex) error {
	checksum := s.Snapshot().Checksum
	if index.Source.Checksum != checksum {
		return fmt.Errorf("attendance item index source mismatch")
	}
	for _, day := range c.Days {
		for _, r := range day.Rewards {
			item, ok := index.Items[r.Item]
			if !ok {
				return fmt.Errorf("attendance reward %d missing from selected item index", r.Item)
			}
			if _, ok = s.archive.FindFile(item.Path); !ok {
				return fmt.Errorf("attendance reward %d script missing", r.Item)
			}
		}
	}
	if _, ok := s.archive.FindFile(c.UI); !ok {
		return fmt.Errorf("attendance UI %s missing", c.UI)
	}
	// The captured 331 opening row names this sibling calendar; the event's
	// [ui path] names its detail window. Support that native family explicitly.
	c.CalendarUI = path.Join(path.Dir(strings.ReplaceAll(c.UI, "\\", "/")), "cal.xui")
	if _, ok := s.archive.FindFile(c.CalendarUI); !ok {
		return fmt.Errorf("attendance calendar UI missing")
	}
	tables, err := s.archive.StringTables()
	if err != nil {
		return err
	}
	texts := map[string]string{}
	seenPaths := map[string]bool{}
	for _, path := range tables {
		if seenPaths[path] {
			continue
		}
		seenPaths[path] = true
		text, err := s.archive.ReadText(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(text, "\n") {
			key, value, found := strings.Cut(strings.TrimRight(line, "\r"), ">")
			key = strings.TrimSpace(strings.TrimPrefix(key, "\ufeff"))
			if !found || (key != c.MailTitle && key != c.MailMessage) {
				continue
			}
			if old, exists := texts[key]; exists && old != value {
				return fmt.Errorf("ambiguous attendance mail key %s", key)
			}
			texts[key] = value
		}
	}
	c.SourceChecksum = checksum
	c.MailTextTitle, c.MailTextMessage = texts[c.MailTitle], texts[c.MailMessage]
	if c.MailTextTitle == "" || c.MailTextMessage == "" {
		return fmt.Errorf("attendance mail text missing from selected source")
	}
	return nil
}

func preparePVFAttendanceActivities(c *Catalogs, s *Source, selected map[string]bool) error {
	if selected["membership"] {
		if c.Items == nil {
			return fmt.Errorf("membership requires items")
		}
		var err error
		c.Membership, err = s.Membership(*c.Items)
		if err != nil {
			return err
		}
	}
	if selected["attendance-daily"] {
		if c.Items == nil {
			return fmt.Errorf("attendance-daily requires items")
		}
		var err error
		c.AttendanceDaily, err = s.RoutineDaily(*c.Items)
		if err != nil {
			return err
		}
	}
	if selected["attendance-online"] {
		if c.Items == nil {
			return fmt.Errorf("attendance-online requires items")
		}
		var err error
		c.AttendanceOnline, err = s.RoutineOnline(*c.Items)
		if err != nil {
			return err
		}
	}
	if selected["attendance-weekly"] {
		if c.Items == nil {
			return fmt.Errorf("attendance-weekly requires items")
		}
		var err error
		c.AttendanceWeekly, err = s.RoutineWeekly(*c.Items)
		if err != nil {
			return err
		}
	}
	if selected["attendance-login"] {
		if c.Items == nil {
			return fmt.Errorf("attendance-login requires items")
		}
		var err error
		c.AttendanceLogin, err = s.AttendanceLogin(*c.Items)
		if err != nil {
			return err
		}
	}
	if !selected["attendance-activities"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("attendance-activities requires items")
	}
	var err error
	c.AttendanceActivities, err = s.AttendanceActivities(*c.Items)
	return err
}
