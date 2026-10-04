package reward

import "context"

type EventType string

const (
	EventLevelUp         EventType = "level_up"
	EventQuestComplete   EventType = "quest_complete"
	EventCharacterCreate EventType = "character_create"
)

// Recipient identifies the character an event/reward belongs to.
type Recipient struct {
	AccountID     int64
	CharacterID   int64
	Name          string
	Level         byte
	ConfigVersion string
}

type Event struct {
	Type      EventType
	Recipient Recipient
	QuestID   uint16
}

type ItemGrant struct {
	ID    uint32
	Count uint32
}

type MailReward struct {
	Subject string
	Body    string
	// Attachments optionally deliver items/equipment with the mail. The mail
	// claim path restores each one into the bag, so a full bag leaves the
	// attachments waiting in the mailbox instead of failing the reward.
	Attachments []ItemGrant
}

// Notifier is the only interface the domains depend on. Domains call these
// after a business success; they never read reward rules themselves.
type Notifier interface {
	LevelUp(ctx context.Context, r Recipient)
	QuestComplete(ctx context.Context, r Recipient, questID uint16)
	CharacterCreate(ctx context.Context, r Recipient)
}
