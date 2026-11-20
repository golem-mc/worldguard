package chat

import "testing"

func TestLegacy(t *testing.T) {
	got := Legacy("§c§lHey!§7 You & me §zx")
	if got.PlainString() != "Hey! You & me §zx" {
		t.Fatalf("plain %q", got.PlainString())
	}
	if len(got.Children) != 2 || got.Children[0].Style.Color != "red" || got.Children[0].Style.Bold == nil ||
		got.Children[1].Style.Color != "gray" || got.Children[1].Style.Bold != nil {
		t.Fatalf("styles %+v", got.Children)
	}
	if Legacy("plain").Literal != "plain" {
		t.Fatal("plain text was split")
	}
}
