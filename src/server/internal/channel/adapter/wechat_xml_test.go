package adapter

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

func TestWeChatAdapterTransformsXMLInboundAndOutbound(t *testing.T) {
	adp := NewWeChatAdapter()

	message, err := adp.TransformInbound(json.RawMessage(`<xml>
		<ToUserName><![CDATA[gh_bot]]></ToUserName>
		<FromUserName><![CDATA[user_openid]]></FromUserName>
		<CreateTime>1717747200</CreateTime>
		<MsgType><![CDATA[text]]></MsgType>
		<Content><![CDATA[hello xml]]></Content>
		<MsgId>wechat_xml_1</MsgId>
	</xml>`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "wechat_xml_1" || message.ConversationID != "user_openid" || message.Role != "user" {
		t.Fatalf("unexpected XML inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Type != "text" || message.Content[0].Text != "hello xml" {
		t.Fatalf("unexpected XML inbound content: %+v", message.Content)
	}
	if message.Metadata["raw_format"] != "xml" || message.Metadata["to_user_name"] != "gh_bot" {
		t.Fatalf("expected XML metadata, got %+v", message.Metadata)
	}

	raw, err := adp.TransformOutbound(InternalMessage{
		ID:             "reply_xml_1",
		ConversationID: "user_openid",
		Role:           "assistant",
		Content:        []ContentPart{{Type: "text", Text: "wechat XML reply"}},
		Metadata: map[string]any{
			"wechat_format":  "xml",
			"from_user_name": "gh_bot",
			"create_time":    int64(1717747201),
		},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload struct {
		XMLName      xml.Name `xml:"xml"`
		ToUserName   string   `xml:"ToUserName"`
		FromUserName string   `xml:"FromUserName"`
		CreateTime   int64    `xml:"CreateTime"`
		MsgType      string   `xml:"MsgType"`
		Content      string   `xml:"Content"`
	}
	if err := xml.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not XML: %v; payload=%s", err, raw)
	}
	if payload.ToUserName != "user_openid" ||
		payload.FromUserName != "gh_bot" ||
		payload.CreateTime != int64(1717747201) ||
		payload.MsgType != "text" ||
		payload.Content != "wechat XML reply" {
		t.Fatalf("unexpected XML outbound payload: %+v from %s", payload, raw)
	}
}
