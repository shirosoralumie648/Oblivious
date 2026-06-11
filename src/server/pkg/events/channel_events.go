package events

const (
	ChannelMessageReceived = "channel.message.received"
	ChannelMessageSent     = "channel.message.sent"
	ChannelMessageFailed   = "channel.message.failed"
	ChannelCreated         = "channel.created"
	ChannelUpdated         = "channel.updated"
	ChannelDeleted         = "channel.deleted"
)

type ChannelMessagePayload struct {
	ChannelID      string
	MessageID      string
	ConversationID string
	Direction      string
	Status         string
	Error          string
}

type ChannelConfigPayload struct {
	ChannelID      string
	OrganizationID string
	Type           string
	Status         string
}
