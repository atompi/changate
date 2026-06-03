package handler

import (
	"testing"

	"github.com/atompi/changate/internal/model"
)

func TestShouldProcessMessage(t *testing.T) {
	tests := []struct {
		name     string
		chatType string
		mentions []model.Mention
		botName  string
		want     bool
	}{
		{
			name:     "DM always processes",
			chatType: "dm",
			mentions: nil,
			botName:  "MyBot",
			want:     true,
		},
		{
			name:     "p2p alias always processes",
			chatType: "p2p",
			mentions: nil,
			botName:  "MyBot",
			want:     true,
		},
		{
			name:     "group without mention is skipped",
			chatType: "group",
			mentions: nil,
			botName:  "MyBot",
			want:     false,
		},
		{
			name:     "group with user mention only is skipped",
			chatType: "group",
			mentions: []model.Mention{
				{Key: "@_user_1", Type: model.MentionTypeUser, Name: "Alice"},
			},
			botName: "MyBot",
			want:    false,
		},
		{
			name:     "group with bot mention matching name is processed",
			chatType: "group",
			mentions: []model.Mention{
				{Key: "@_user_2", Type: model.MentionTypeBot, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    true,
		},
		{
			name:     "channel with bot mention is processed",
			chatType: "channel",
			mentions: []model.Mention{
				{Key: "@_user_1", Type: model.MentionTypeBot, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    true,
		},
		{
			name:     "group with different bot name is skipped",
			chatType: "group",
			mentions: []model.Mention{
				{Key: "@_user_1", Type: model.MentionTypeBot, Name: "OtherBot"},
			},
			botName: "MyBot",
			want:    false,
		},
		{
			name:     "thread is treated like group",
			chatType: "thread",
			mentions: nil,
			botName:  "MyBot",
			want:     false,
		},
		{
			name:     "group with both user and bot mentions processes",
			chatType: "group",
			mentions: []model.Mention{
				{Key: "@_user_1", Type: model.MentionTypeUser, Name: "Alice"},
				{Key: "@_user_2", Type: model.MentionTypeBot, Name: "MyBot"},
			},
			botName: "MyBot",
			want:    true,
		},
		{
			name:     "group with empty botName is always skipped (no match possible)",
			chatType: "group",
			mentions: []model.Mention{
				{Key: "@_user_1", Type: model.MentionTypeBot, Name: "MyBot"},
			},
			botName: "",
			want:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := model.MessageInfo{ChatType: tc.chatType, Mentions: tc.mentions}
			if got := shouldProcessMessage(msg, tc.botName); got != tc.want {
				t.Errorf("shouldProcessMessage() = %v, want %v", got, tc.want)
			}
		})
	}
}
