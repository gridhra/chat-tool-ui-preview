// Package model は投稿の内部モデル（AST）を定義する。入力形式（標準Markdown / Slack記法mrkdwn / Block Kit JSON）に
// 依らず、描画はこのモデルだけを見る。
package model

import "encoding/json"

// InlineKind は行内要素の種類。
type InlineKind string

const (
	InlineText    InlineKind = "text"
	InlineBold    InlineKind = "bold"
	InlineItalic  InlineKind = "italic"
	InlineStrike  InlineKind = "strike"
	InlineCode    InlineKind = "code"
	InlineLink    InlineKind = "link"
	InlineMention InlineKind = "mention"
	InlineEmoji   InlineKind = "emoji"
	InlineDate    InlineKind = "date"
	InlineBreak   InlineKind = "br"
)

// MentionKind はメンションの種類。
type MentionKind string

const (
	MentionUser       MentionKind = "user"
	MentionChannel    MentionKind = "channel"
	MentionSubteam    MentionKind = "subteam"
	MentionHere       MentionKind = "here"
	MentionChannelAll MentionKind = "channel_all"
	MentionEveryone   MentionKind = "everyone"
)

// Inline は行内要素。Kindに応じて使うフィールドが変わる。
type Inline struct {
	Kind      InlineKind
	Value     string   // text / code
	Children  []Inline // bold / italic / strike / link
	URL       string   // link
	Mention   MentionKind
	ID        string // mention
	Label     string // mention
	Resolved  bool   // mention: 表示名が分かっているか
	Name      string // emoji
	Unicode   string // emoji（空なら未知）
	Timestamp int64  // date
	Format    string // date
	Fallback  string // date
}

// ElementKind は操作部品の種類。プレビューでは「ここに部品がある」と示すだけ。
type ElementKind string

const (
	ElementButton      ElementKind = "button"
	ElementImage       ElementKind = "image"
	ElementPlaceholder ElementKind = "placeholder"
)

// Element はボタン・選択肢などの操作部品。
type Element struct {
	Kind  ElementKind
	Text  string // button
	Style string // button: "" | primary | danger
	URL   string // button / image
	Alt   string // image
	What  string // placeholder: 元の要素型名
	Label string // placeholder: placeholder文
}

// BlockKind はブロックの種類。
type BlockKind string

const (
	BlockParagraph   BlockKind = "paragraph"
	BlockHeading     BlockKind = "heading"
	BlockQuote       BlockKind = "quote"
	BlockCode        BlockKind = "code_block"
	BlockList        BlockKind = "list"
	BlockTable       BlockKind = "table"
	BlockDivider     BlockKind = "divider"
	BlockSection     BlockKind = "section"
	BlockContext     BlockKind = "context"
	BlockImage       BlockKind = "image"
	BlockActions     BlockKind = "actions"
	BlockUnsupported BlockKind = "unsupported"
)

// ContextElement は context ブロックの要素。Text が非nilなら文、そうでなければ Element。
type ContextElement struct {
	Text    []Block
	Element *Element
}

// Block はブロック要素。Kindに応じて使うフィールドが変わる。
type Block struct {
	Kind      BlockKind
	Children  []Inline   // paragraph / heading
	Level     int        // heading
	Blocks    []Block    // quote
	Value     string     // code_block
	Lang      string     // code_block
	Ordered   bool       // list
	Items     [][]Block  // list
	Header    [][]Inline // table
	Rows      [][][]Inline
	Text      []Block          // section
	Fields    [][]Block        // section
	Accessory *Element         // section
	Elements  []ContextElement // context
	Actions   []Element        // actions
	URL       string           // image
	Alt       string           // image
	Title     string           // image
	What      string           // unsupported
}

// WarningCode は警告の種類。
type WarningCode string

const (
	WarnUnresolvedMention WarningCode = "unresolved_mention"
	WarnUnknownEmoji      WarningCode = "unknown_emoji"
	WarnTooLong           WarningCode = "too_long"
	WarnInvalidBlock      WarningCode = "invalid_block"
	WarnUnsupported       WarningCode = "unsupported"
	WarnConversionNote    WarningCode = "conversion_note"
	WarnMissingContext    WarningCode = "missing_context"
	WarnBroadcast         WarningCode = "broadcast"
)

// Warning は「確認が必要な点」。
type Warning struct {
	Code    WarningCode `json:"code"`
	Message string      `json:"message"`
}

// Format は入力形式。推定はしない。
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatMrkdwn   Format = "mrkdwn"
	FormatBlocks   Format = "blocks"
)

// Source は入力の出所。注記の出し分けに使う。
type Source string

const (
	SourceClaudeSlackMCP Source = "claude-slack-mcp"
	SourceSlackWebAPI    Source = "slack-web-api"
	SourceManual         Source = "manual"
)

// PostContext は投稿の文脈（宛先・スレッド・送信主体）。
type PostContext struct {
	ChannelID      string            `json:"channel_id,omitempty"`
	ChannelName    string            `json:"channel_name,omitempty"`
	ThreadTS       string            `json:"thread_ts,omitempty"`
	ReplyBroadcast bool              `json:"reply_broadcast,omitempty"`
	As             string            `json:"as,omitempty"` // "user" | "bot"
	SenderName     string            `json:"sender_name,omitempty"`
	Names          map[string]string `json:"names,omitempty"` // ID→表示名
}

// Input は核が受け取る入力JSON。
type Input struct {
	Target  string            `json:"target"`
	Format  Format            `json:"format"`
	Text    string            `json:"text,omitempty"`
	Blocks  []json.RawMessage `json:"blocks,omitempty"`
	Context PostContext       `json:"context"`
	Source  Source            `json:"source,omitempty"`
}

// Document は描画の入力。
type Document struct {
	Target   string
	Format   Format
	Source   Source
	Context  PostContext
	Blocks   []Block
	Warnings []Warning
}

// Text は文字列だけの行内要素を作る。
func Text(s string) Inline { return Inline{Kind: InlineText, Value: s} }

// Paragraph は段落を作る。
func Paragraph(children []Inline) Block { return Block{Kind: BlockParagraph, Children: children} }
