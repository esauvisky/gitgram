package render

import (
	"hash/fnv"

	"github.com/esauvisky/gitgram/internal/render/htmlfmt"
)

// taglines close one card in taglineOdds with a line of developer humour
// anyone in the trade will recognise.
var taglines = []string{
	"It works on my machine. Ship the machine.",
	"Two hard things in computer science: cache invalidation, naming things and off-by-one errors.",
	"A programmer had a problem and reached for async. Problems two has now he.",
	"Some people see a problem and think: regex! Now they have two problems.",
	"99 little bugs in the code. Patch one, compile again: 127 little bugs in the code.",
	"There are 10 kinds of people: those who read binary and those who don't.",
	"My code's compiling. That's the official excuse.",
	"It's not a bug, it's an undocumented feature.",
	"If debugging removes bugs, programming must be how they get in.",
	"Weeks of coding can save you hours of planning.",
	"The first 90% of the code takes 90% of the time. The other 10% takes the other 90%.",
	"Nothing is more permanent than a temporary fix.",
	"All tests pass. All zero of them.",
	"Deploying on a Friday: bold, brave, regrettable.",
	"To understand recursion, first understand recursion.",
	"TODO: remove this TODO.",
	"Have you tried turning it off and on again?",
	"A QA engineer walks into a bar and orders -1 beers.",
	"Code never lies. Comments sometimes do.",
	"The rubber duck is still the best senior engineer on the team.",
	"Merge conflicts are git asking how your day is going.",
	"Forecast: 100% chance of \"it was working yesterday\".",
	"The cloud is just someone else's computer.",
	"Every project starts with git init and ends with git blame.",
	"We'll fix it next sprint. (We won't.)",
	"Hours of debugging can save you minutes of reading the docs.",
	"It compiles. Ship it.",
	"In theory, theory and practice are the same. In practice, they aren't.",
	"Legacy code: it works, and the person who knew why left.",
	"Real programmers count from 0.",
	"Copied from Stack Overflow, with love and no attribution.",
	"Fixed one bug. Two more volunteered.",
	"The code is self-documenting. It documents confusion.",
	"Commit message: \"fix\". Previous commit message: \"fix\".",
	"Debugging: being the detective in a crime movie where you're also the murderer.",
	"Production is the best test environment. Nobody agrees.",
	"There's no place like 127.0.0.1.",
	"Estimate: two days. Actual: two sprints and a rewrite.",
	"rm -rf node_modules and pray.",
	"Undefined is not a function, and today neither am I.",
}

// taglineOdds is the share of cards that get a tagline: one in ten.
const taglineOdds = 10

// taglineFooter closes one card in taglineOdds with a blank line and a
// tagline in italics. Both the draw and the line come from a hash of the
// card's seed, so a card keeps its tagline (or its silence) across edits
// and different cards get different lines.
func taglineFooter(b *htmlfmt.Builder, seed string) {
	h := fnv.New64a()
	h.Write([]byte(seed))
	n := h.Sum64()
	if n%taglineOdds != 0 {
		return
	}
	b.Line("")
	b.Line(htmlfmt.I(taglines[(n/taglineOdds)%uint64(len(taglines))]))
}
