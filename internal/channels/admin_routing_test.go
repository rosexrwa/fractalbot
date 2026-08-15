package channels

import (
	"fmt"
	"testing"
	"time"

	"github.com/fractalmind-ai/fractalbot/pkg/protocol"
)

func TestAdminCommandCompletenessAcrossChannelGuards(t *testing.T) {
	guards := map[string]func(string) bool{
		"slack":   isIncompleteSlackAgentCommand,
		"discord": isIncompleteDiscordAgentCommand,
		"feishu":  isIncompleteFeishuAgentCommand,
	}
	tests := []struct {
		text       string
		incomplete bool
	}{
		{text: "/admin", incomplete: true},
		{text: "/admin@fractalbot", incomplete: true},
		{text: "/admin recover", incomplete: false},
		{text: "/agent admin", incomplete: true},
		{text: "/agent admin recover", incomplete: false},
		{text: "/to admin recover", incomplete: false},
		{text: "/administrator recover", incomplete: false},
	}

	for channel, guard := range guards {
		for _, tt := range tests {
			t.Run(channel+"_"+tt.text, func(t *testing.T) {
				if got := guard(tt.text); got != tt.incomplete {
					t.Fatalf("guard(%q)=%v, want %v", tt.text, got, tt.incomplete)
				}
			})
		}
	}
}

func TestAdminRoutingPayloadAcrossChannelAdapters(t *testing.T) {
	const rawText = "/admin recover main session"
	selection, err := ParseAgentSelection(rawText)
	if err != nil {
		t.Fatalf("ParseAgentSelection: %v", err)
	}
	if selection.Agent != AdminAgentName || selection.Task != "recover main session" || !selection.Specified {
		t.Fatalf("unexpected selection: %+v", selection)
	}

	eventTime := time.Date(2026, 7, 3, 4, 5, 6, 0, time.UTC)
	cases := []struct {
		channel string
		build   func() *protocol.Message
	}{
		{
			channel: "telegram",
			build: func() *protocol.Message {
				return (&TelegramBot{}).convertToProtocolMessage(&TelegramMessage{
					MessageID:       101,
					MessageThreadID: 202,
					From:            &TelegramUser{ID: 303, UserName: "operator"},
					Chat:            &TelegramChat{ID: 404, Type: "private"},
					Date:            eventTime.Unix(),
					Text:            rawText,
				}, selection.Task, selection.Agent, nil)
			},
		},
		{
			channel: "slack",
			build: func() *protocol.Message {
				return (&SlackBot{}).toProtocolMessage(&slackInboundMessage{
					text:        rawText,
					userID:      "U303",
					channelID:   "C404",
					channelType: "im",
					threadTS:    "1719999999.000200",
					timestamp:   "1719999999.000100",
				}, selection.Task, selection.Agent, "full", nil)
			},
		},
		{
			channel: "discord",
			build: func() *protocol.Message {
				return (&DiscordBot{}).toProtocolMessage(&discordInboundMessage{
					text:        rawText,
					userID:      "303",
					channelID:   "404",
					channelType: "dm",
					messageID:   "101",
					timestamp:   eventTime,
				}, selection.Task, selection.Agent)
			},
		},
		{
			channel: "feishu",
			build: func() *protocol.Message {
				return (&FeishuBot{}).toProtocolMessage(&feishuInboundMessage{
					text:      rawText,
					userID:    "ou_303",
					chatID:    "oc_404",
					chatType:  "p2p",
					messageID: "om_101",
					threadID:  "omt_202",
					timestamp: "1783051506000",
				}, selection.Task, selection.Agent)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.channel, func(t *testing.T) {
			msg := tt.build()
			data, ok := msg.Data.(map[string]interface{})
			if !ok {
				t.Fatalf("payload type=%T", msg.Data)
			}
			assertPayloadValue(t, data, "channel", tt.channel)
			assertPayloadValue(t, data, "text", selection.Task)
			assertPayloadValue(t, data, "raw_text", rawText)
			assertPayloadValue(t, data, "agent", AdminAgentName)
			for _, key := range []string{"chat_id", "user_id", "timestamp"} {
				if fmt.Sprint(data[key]) == "" || data[key] == nil {
					t.Fatalf("missing %s in payload: %#v", key, data)
				}
			}
		})
	}
}

func TestIMessageAdminPayloadPreservesManagerRoutingContext(t *testing.T) {
	const rawText = "/admin recover main session"
	msg := (&IMessageBot{}).toProtocolMessage(IMessageInbound{
		MessageID: 101,
		Sender:    "+15551234567",
		Text:      rawText,
		Timestamp: time.Date(2026, 7, 3, 4, 5, 6, 0, time.UTC),
	})
	data := msg.Data.(map[string]interface{})
	assertPayloadValue(t, data, "channel", "imessage")
	assertPayloadValue(t, data, "text", rawText)
	assertPayloadValue(t, data, "raw_text", rawText)
	assertPayloadValue(t, data, "chat_id", "+15551234567")
	assertPayloadValue(t, data, "user_id", "+15551234567")
	if fmt.Sprint(data["timestamp"]) == "" {
		t.Fatalf("missing timestamp: %#v", data)
	}

	selection, ok, err := ParseAdminSelection(fmt.Sprint(data["text"]))
	if err != nil || !ok || selection.Agent != AdminAgentName || selection.Task != "recover main session" {
		t.Fatalf("unexpected manager selection: selection=%+v ok=%v err=%v", selection, ok, err)
	}
}

func assertPayloadValue(t *testing.T, data map[string]interface{}, key, want string) {
	t.Helper()
	if got := fmt.Sprint(data[key]); got != want {
		t.Fatalf("%s=%q, want %q (payload=%#v)", key, got, want, data)
	}
}
