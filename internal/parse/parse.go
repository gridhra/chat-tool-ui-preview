package parse

import (
	"fmt"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

// Build は入力JSON → 文書モデル。形式は `format` で明示されたものだけを使い、推定しない。
func Build(in model.Input) (model.Document, error) {
	var warnings []model.Warning
	o := &Options{Names: in.Context.Names, Warnings: &warnings}
	source := in.Source
	if source == "" {
		source = model.SourceManual
	}
	var blocks []model.Block
	switch in.Format {
	case model.FormatMarkdown:
		blocks = Markdown(in.Text, o)
		if source == model.SourceClaudeSlackMCP {
			o.warn(model.WarnConversionNote, "この本文は標準Markdownで、送信先の記法への変換は送信時に送信先側で行われる。表・見出し・入れ子の箇条書きは実物と見え方が違うことがある")
		}
	case model.FormatMrkdwn:
		blocks = MrkdwnBlocks(in.Text, o)
	case model.FormatBlocks:
		blocks = BlockKit(in.Blocks, o)
		if in.Text == "" {
			o.warn(model.WarnConversionNote, "blocks のみで text（通知やプッシュに使われる代替文）が無い")
		}
	default:
		return model.Document{}, fmt.Errorf("format が不正: %q（markdown | mrkdwn | blocks のどれか）", in.Format)
	}
	if in.Context.ChannelID == "" {
		o.warn(model.WarnMissingContext, "宛先（channel_id）が無い")
	}
	target := in.Target
	if target == "" {
		target = "slack"
	}
	return model.Document{Target: target, Format: in.Format, Source: source, Context: in.Context, Blocks: blocks, Warnings: dedupe(warnings)}, nil
}

func dedupe(ws []model.Warning) []model.Warning {
	seen := map[model.Warning]bool{}
	out := make([]model.Warning, 0, len(ws))
	for _, w := range ws {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}
