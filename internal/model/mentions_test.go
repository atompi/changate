package model

import (
	"testing"
)

func TestIsBotMentioned(t *testing.T) {
	tests := []struct {
		name     string
		mentions []Mention
		botName  string
		want     bool
	}{
		{
			name:     "no mentions",
			mentions: nil,
			botName:  "MyBot",
			want:     false,
		},
		{
			name: "empty botName never matches",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeBot, Name: "MyBot"},
			},
			botName: "",
			want:    false,
		},
		{
			name: "user-type mention only",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeUser, Name: "Alice"},
			},
			botName: "MyBot",
			want:    false,
		},
		{
			name: "bot mention with matching name",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeUser, Name: "Alice"},
				{Key: "@_user_2", Type: MentionTypeBot, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    true,
		},
		{
			name: "bot mention with non-matching name",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeBot, Name: "OtherBot"},
			},
			botName: "MyBot",
			want:    false,
		},
		{
			name: "type missing treated as not bot",
			mentions: []Mention{
				{Key: "@_user_1", Name: "MyBot"},
			},
			botName: "MyBot",
			want:    false,
		},
		{
			name: "case sensitive matching",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeBot, Name: "mybot"},
			},
			botName: "MyBot",
			want:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBotMentioned(tc.mentions, tc.botName); got != tc.want {
				t.Errorf("IsBotMentioned() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBotMentionKey(t *testing.T) {
	tests := []struct {
		name     string
		mentions []Mention
		botName  string
		want     string
	}{
		{
			name:     "no mentions",
			mentions: nil,
			botName:  "MyBot",
			want:     "",
		},
		{
			name: "returns key for matching bot",
			mentions: []Mention{
				{Key: "@_user_9", Type: MentionTypeBot, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    "@_user_9",
		},
		{
			name: "skips non-matching bots",
			mentions: []Mention{
				{Key: "@_user_7", Type: MentionTypeBot, Name: "OtherBot"},
				{Key: "@_user_8", Type: MentionTypeBot, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    "@_user_8",
		},
		{
			name: "empty botName returns empty",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeBot, Name: "MyBot"},
			},
			botName: "",
			want:    "",
		},
		{
			name: "skips user-type mentions even if name matches",
			mentions: []Mention{
				{Key: "@_user_1", Type: MentionTypeUser, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BotMentionKey(tc.mentions, tc.botName); got != tc.want {
				t.Errorf("BotMentionKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripBotMention(t *testing.T) {
	tests := []struct {
		name string
		text string
		key  string
		want string
	}{
		{
			name: "empty text",
			text: "",
			key:  "@_user_1",
			want: "",
		},
		{
			name: "empty key",
			text: "  hello  ",
			key:  "",
			want: "hello",
		},
		{
			name: "key not present",
			text: "hello world",
			key:  "@_user_1",
			want: "hello world",
		},
		{
			name: "key at start of text",
			text: "@_user_1 hello",
			key:  "@_user_1",
			want: "hello",
		},
		{
			name: "key at end of text",
			text: "hello @_user_1",
			key:  "@_user_1",
			want: "hello",
		},
		{
			name: "key in middle of text",
			text: "hi @_user_1 how are you",
			key:  "@_user_1",
			want: "hi  how are you",
		},
		{
			name: "trims surrounding whitespace after strip",
			text: "  @_user_1  hello  ",
			key:  "@_user_1",
			want: "hello",
		},
		{
			name: "chinese text with mention",
			text: "@_user_1 你好",
			key:  "@_user_1",
			want: "你好",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripBotMention(tc.text, tc.key); got != tc.want {
				t.Errorf("StripBotMention(%q, %q) = %q, want %q", tc.text, tc.key, got, tc.want)
			}
		})
	}
}

func TestMessageInfo_IsDM(t *testing.T) {
	tests := []struct {
		chatType string
		want     bool
	}{
		{"dm", true},
		{"p2p", true},
		{"group", false},
		{"channel", false},
		{"thread", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.chatType, func(t *testing.T) {
			m := MessageInfo{ChatType: tc.chatType}
			if got := m.IsDM(); got != tc.want {
				t.Errorf("IsDM() for %q = %v, want %v", tc.chatType, got, tc.want)
			}
		})
	}
}
