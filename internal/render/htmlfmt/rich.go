package htmlfmt

import "strings"

// RichLimit is the rich message text limit in UTF-8 characters, with room
// kept for entity overhead.
const RichLimit = 30000

// Doc accumulates rich message HTML block by block: headings, paragraphs,
// tables, details, footers. Blocks are the unit of truncation.
type Doc struct {
	blocks []string
}

// Block appends one complete block.
func (d *Doc) Block(html string) {
	if html == "" {
		return
	}
	if n := len(d.blocks); html == Divider() && n > 0 && d.blocks[n-1] == Divider() {
		return
	}
	d.blocks = append(d.blocks, html)
}

// Last returns the last block appended, "" when none.
func (d *Doc) Last() string {
	if len(d.blocks) == 0 {
		return ""
	}
	return d.blocks[len(d.blocks)-1]
}

// P appends a paragraph.
func (d *Doc) P(inner string) { d.Block("<p>" + inner + "</p>") }

// String joins the blocks; when the whole exceeds RichLimit, blocks are
// dropped from the end until it fits.
func (d *Doc) String() string {
	blocks := d.blocks
	for len(blocks) > 1 && size(blocks) > RichLimit {
		blocks = blocks[:len(blocks)-1]
	}
	return strings.Join(blocks, "")
}

func size(blocks []string) int {
	n := 0
	for _, b := range blocks {
		n += len(b) + 1
	}
	return n
}

// Heading renders a section heading; size 1 is the largest, 6 the smallest.
func Heading(inner string, size int) string {
	n := string(rune('0' + size))
	return "<h" + n + ">" + inner + "</h" + n + ">"
}

// Divider renders a horizontal rule block.
func Divider() string { return "<hr/>" }

// Footer renders the small footer block.
func Footer(inner string) string { return "<footer>" + inner + "</footer>" }

// Details renders a collapsible block with an always-visible summary.
func Details(summary, inner string, open bool) string {
	tag := "<details>"
	if open {
		tag = "<details open>"
	}
	return tag + "<summary>" + summary + "</summary>" + inner + "</details>"
}

// Table renders a compact table from rows of already-formatted cells; the
// last column is right-aligned.
func Table(rows [][]string) string {
	var sb strings.Builder
	sb.WriteString("<table compact>")
	for _, row := range rows {
		sb.WriteString("<tr>")
		for i, cell := range row {
			if i == len(row)-1 && len(row) > 1 {
				sb.WriteString(`<td align="right">` + cell + "</td>")
			} else {
				sb.WriteString("<td>" + cell + "</td>")
			}
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</table>")
	return sb.String()
}
