// Package markdown converts between normalized Notion blocks and markdown.
// Backend-agnostic: it works with notion.NormalizedBlock, so both the official
// REST and v3 internal backends share the same rendering. Ported from the
// TypeScript reference (bun/src/notion/markdown.ts) to stay byte-for-byte
// compatible for differential parity.
package markdown

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/shhac/agent-notion/internal/notion"
)

// blockToMarkdown converts a single normalized block to a markdown line.
//
// The TS reference uses `??` (nullish) and `||` (falsy) fallbacks. Optional
// strings arrive here as "" when absent (the TS used undefined), so both
// collapse to "treat empty as missing": firstNonEmpty covers the `||` chains,
// and the `?? "default"` cases fall back whenever the field is empty.
func blockToMarkdown(b notion.NormalizedBlock, indent string) string {
	text := b.RichText

	switch b.Type {
	case "paragraph":
		if text != "" {
			return indent + text
		}
		return ""
	case "heading_1":
		return indent + "# " + text
	case "heading_2":
		return indent + "## " + text
	case "heading_3":
		return indent + "### " + text
	case "bulleted_list_item":
		return indent + "- " + text
	case "numbered_list_item":
		return indent + "1. " + text
	case "to_do":
		checked := " "
		if b.Checked != nil && *b.Checked {
			checked = "x"
		}
		return indent + "- [" + checked + "] " + text
	case "toggle":
		return indent + "> ▶ " + text
	case "code":
		return indent + "```" + b.Language + "\n" + text + "\n" + indent + "```"
	case "quote":
		return indent + "> " + text
	case "callout":
		return indent + "> " + firstNonEmpty(b.Emoji, "💡") + " " + text
	case "divider":
		return indent + "---"
	case "image":
		return indent + "![" + firstNonEmpty(b.Caption, "image") + "](" + b.URL + ")"
	case "bookmark":
		return indent + "[" + firstNonEmpty(b.Caption, b.URL, "bookmark") + "](" + b.URL + ")"
	case "equation":
		return indent + "$$" + b.Expression + "$$"
	case "child_page":
		return indent + "📄 " + firstNonEmpty(b.Title, "Untitled")
	case "child_database":
		return indent + "📊 " + firstNonEmpty(b.Title, "Untitled")
	case "table_of_contents":
		return indent + "[Table of Contents]"
	case "breadcrumb":
		return indent + "[Breadcrumb]"
	case "column_list", "column", "synced_block":
		return ""
	case "link_preview":
		return indent + "[" + firstNonEmpty(b.URL, "link") + "](" + b.URL + ")"
	case "embed":
		return indent + "[embed: " + b.URL + "](" + b.URL + ")"
	case "video":
		return indent + "[video](" + b.URL + ")"
	case "pdf":
		return indent + "[pdf](" + b.URL + ")"
	case "audio":
		return indent + "[audio](" + b.URL + ")"
	case "file":
		return indent + "[" + firstNonEmpty(b.Caption, b.Title, "file") + "](" + b.URL + ")"
	default:
		if text != "" {
			return indent + text
		}
		return indent + "[unsupported: " + b.Type + "]"
	}
}

// firstNonEmpty returns the first non-empty string, mirroring the TS `||`/`??`
// fallback chains.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// FromBlocks converts a slice of normalized blocks to markdown. childBlocksMap
// supplies nested content keyed by parent block ID (nil is fine when there is
// no nesting). Non-empty lines are joined with blank lines between them.
func FromBlocks(blocks []notion.NormalizedBlock, childBlocksMap map[string][]notion.NormalizedBlock) string {
	return fromBlocks(blocks, childBlocksMap, "")
}

func fromBlocks(blocks []notion.NormalizedBlock, childBlocksMap map[string][]notion.NormalizedBlock, indent string) string {
	lines := make([]string, 0, len(blocks))

	for _, block := range blocks {
		lines = append(lines, blockToMarkdown(block, indent))

		if block.HasChildren {
			if children, ok := childBlocksMap[block.ID]; ok {
				childMd := fromBlocks(children, childBlocksMap, indent+"  ")
				if childMd != "" {
					lines = append(lines, childMd)
				}
			}
		}
	}

	nonEmpty := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

// FlattenBlock reduces a normalized block to a simplified object for --raw
// output. The "content" key is omitted when the block has no rich text (the TS
// set it to undefined, which JSON drops). Keys are snake_case per the domain
// contract (an intended break from the TS camelCase).
func FlattenBlock(block notion.NormalizedBlock) map[string]any {
	flat := map[string]any{
		"id":           block.ID,
		"type":         block.Type,
		"has_children": block.HasChildren,
	}
	if block.RichText != "" {
		flat["content"] = block.RichText
	}
	return flat
}

var (
	headingRe  = regexp.MustCompile(`^(#{1,3})\s+(.+)$`)
	dividerRe  = regexp.MustCompile(`^(-{3,}|\*{3,})$`)
	todoRe     = regexp.MustCompile(`^[-*]\s+\[([ xX])\]\s+(.+)$`)
	bulletRe   = regexp.MustCompile(`^[-*]\s+(.+)$`)
	numberedRe = regexp.MustCompile(`^\d+\.\s+(.+)$`)
	quoteRe    = regexp.MustCompile(`^>\s+(.+)$`)
)

// ToBlocks parses markdown text into Notion block objects (for append). The
// returned maps are in official-API format: each has a "type" key and a
// same-named payload key. It recognizes fenced code, ATX headings (levels
// 1-3), thematic-break dividers, to-do items, bulleted and numbered lists,
// blockquotes, and falls back to paragraphs.
func ToBlocks(markdown string) []map[string]any {
	blocks := []map[string]any{}
	lines := strings.Split(markdown, "\n")
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines.
		if trimmed == "" {
			i++
			continue
		}

		// Fenced code block.
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			codeLines := []string{}
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				codeLines = append(codeLines, lines[i])
				i++
			}
			i++ // skip closing ```
			blocks = append(blocks, map[string]any{
				"type": "code",
				"code": map[string]any{
					"rich_text": richText(strings.Join(codeLines, "\n")),
					"language":  firstNonEmpty(lang, "plain text"),
				},
			})
			continue
		}

		// Headings (levels 1-3, guaranteed by the regex).
		if m := headingRe.FindStringSubmatch(line); m != nil {
			blockType := "heading_" + strconv.Itoa(len(m[1]))
			blocks = append(blocks, map[string]any{
				"type": blockType,
				blockType: map[string]any{
					"rich_text": richText(m[2]),
				},
			})
			i++
			continue
		}

		// Divider.
		if dividerRe.MatchString(trimmed) {
			blocks = append(blocks, map[string]any{
				"type":    "divider",
				"divider": map[string]any{},
			})
			i++
			continue
		}

		// Todo.
		if m := todoRe.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, map[string]any{
				"type": "to_do",
				"to_do": map[string]any{
					"rich_text": richText(m[2]),
					"checked":   m[1] != " ",
				},
			})
			i++
			continue
		}

		// Bulleted list.
		if m := bulletRe.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, map[string]any{
				"type": "bulleted_list_item",
				"bulleted_list_item": map[string]any{
					"rich_text": richText(m[1]),
				},
			})
			i++
			continue
		}

		// Numbered list.
		if m := numberedRe.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, map[string]any{
				"type": "numbered_list_item",
				"numbered_list_item": map[string]any{
					"rich_text": richText(m[1]),
				},
			})
			i++
			continue
		}

		// Blockquote.
		if m := quoteRe.FindStringSubmatch(line); m != nil {
			blocks = append(blocks, map[string]any{
				"type": "quote",
				"quote": map[string]any{
					"rich_text": richText(m[1]),
				},
			})
			i++
			continue
		}

		// Default: paragraph.
		blocks = append(blocks, map[string]any{
			"type": "paragraph",
			"paragraph": map[string]any{
				"rich_text": richText(line),
			},
		})
		i++
	}

	return blocks
}

// richText builds the single-run rich_text array the official API expects.
func richText(content string) []map[string]any {
	return []map[string]any{
		{"type": "text", "text": map[string]any{"content": content}},
	}
}
