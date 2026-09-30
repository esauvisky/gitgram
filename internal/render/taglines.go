package render

import (
	"hash/fnv"
	"strings"

	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// taglines close one card in taglineOdds with a line of house humour.
var taglines = []string{
	"[]]][[][]]]]]][] is PokemonProto and we all pretend that's fine",
	"git blame ruined my day, and it said Cyber",
	"// if it verks, don't touch it",
	"\"Temporarily hardcoded\" (in 2021)",
	"We don't test in prod, prod tests us",
	"Four hours on one SQL query, in prod",
	"Status 200 OK, IVs: 0/0/0",
	"Two hard things: naming things, and Niantic naming two APKs 223.0",
	"99 little bugs, one hotfix to prod, forced update, 127 little bugs",
	"Code quality is measured in PEBCAKs/minute",
	"Beta poll all green, BSOD in prod",
	"\"Works here. Somehow.\"",
	"Bracket obfuscation: rewrote the regex with more brackets",
	"Mozart's cert expired, the email went to Wagner",
	"git commit -m \"[deploy]\" --allow-empty",
	"The bug only happens around Go Fest, which is always",
	"Patreon sync ETA: NaN hours",
	"\"How the hell did I fix it\"",
	"It passes broto-stable. Ship it.",
	"Two yarn dev running, different versions, zero regrets",
	"ETA: :csoon1::csoon2::csoon3:",
	"emi takes a day off, prod goes down",
	"0 days without drama. Record: 0 days.",
	"\"I wiped dev.\" It was prod.",
	"Not a bug, a bugture",
	"Fixed Velocity by adding one s",
	"Bot before GTA 6",
	"\"Sorry. Another one will come.\"",
}

// taglineOdds is the share of cards that get a tagline: one in ten.
const taglineOdds = 10

// taglineFooter closes one card in taglineOdds with a blank line (two
// after a fold, whose own border sits tight under it), a separator, then
// a tagline in italics. Both the draw and the line come from a hash of the
// card's seed, so a card keeps its tagline (or its silence) across edits
// and different cards get different lines. The blank is a paragraph
// holding a braille blank, the one glyph clients keep while showing
// nothing.
func taglineFooter(d *htmlfmt.Doc, seed string) {
	h := fnv.New64a()
	h.Write([]byte(seed))
	n := h.Sum64()
	if n%taglineOdds != 0 {
		return
	}
	if strings.HasPrefix(d.Last(), "<details") {
		d.P("\u2800")
	}
	d.P("\u2800")
	d.Block(htmlfmt.Divider())
	d.Block(htmlfmt.Footer(htmlfmt.I(taglines[(n/taglineOdds)%uint64(len(taglines))])))
}
