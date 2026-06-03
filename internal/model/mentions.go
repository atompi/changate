package model

import "strings"

const (
	MentionTypeBot  = "bot"
	MentionTypeUser = "user"
)

// IsBotMentioned reports whether any mention in the slice is a bot-type
// mention whose Name matches botName. If botName is empty, no mention is
// treated as the bot (the caller has not configured who the bot is, so
// the safe default is to skip).
func IsBotMentioned(mentions []Mention, botName string) bool {
	if botName == "" {
		return false
	}
	for i := range mentions {
		m := &mentions[i]
		if m.Type == "" && m.Name == botName {
			return true
		}
		if m.Type == MentionTypeBot && m.Name == botName {
			return true
		}
	}
	return false
}

// BotMentionKey returns the in-text placeholder (e.g. "@_user_1") for the
// bot's @mention, or "" if the bot is not mentioned.
func BotMentionKey(mentions []Mention, botName string) string {
	if botName == "" {
		return ""
	}
	for i := range mentions {
		m := &mentions[i]
		if (m.Type == "" || m.Type == MentionTypeBot) && m.Name == botName {
			return m.Key
		}
	}
	return ""
}

// StripBotMention removes the bot's @mention placeholder from text and trims
// any leftover leading whitespace, so the Agent receives clean input.
// The key is matched case-sensitively as a whole-word placeholder.
func StripBotMention(text, key string) string {
	if text == "" || key == "" {
		return strings.TrimSpace(text)
	}
	before, _, ok := strings.Cut(text, key)
	if !ok {
		return strings.TrimSpace(text)
	}
	after := text[len(before)+len(key):]
	return strings.TrimSpace(before + after)
}
