---
target: the cards (internal/render)
total_score: 22
max_score: 36
na_heuristics: 10
p0_count: 0
p1_count: 3
target_identity: "file:/home/emi/Coding/gitgram/internal/render"
timestamp: 2026-09-30T07-50-28Z
slug: internal-render
---
Method: dual-agent (A: design review · B: detector)

## Design Health Score
| # | Heuristic | Score | Key Issue |
|---|---|---|---|
| 1 | Visibility of System Status | 3 | Lamps, live tails, in-place edits strong; bot reaction drifted against the text on three of three terminal cards in live captures. |
| 2 | Match System / Real World | 3 | `(script failure)` says nothing; `1 approvals`; `Waiting for manual`. |
| 3 | User Control and Freedom | 2 | Manual production deploy is one tap, no confirm; play reaction fires every manual job. |
| 4 | Consistency and Standards | 3 | Push title uses ` • `, others ` · `; mixed button label grammar; stage link target depends on sort. |
| 5 | Error Prevention | 1 | `▶️ deploy:prod` one tap; stray 🔥 reaction runs a pipeline. |
| 6 | Recognition Rather Than Recall | 3 | Six not-running glyphs to learn; reactions as commands undiscoverable. |
| 7 | Flexibility and Efficiency | 3 | Verbosity levels, wide likely action; no way to quiet a running tail. |
| 8 | Aesthetic and Minimalist Design | 2 | Failed job named three times; anchor linked twice; branch said three times; running card opens ten lines of upload noise. |
| 9 | Error Recovery | 2 | The error line is inside a closed fold labelled by line count. |
| 10 | Help and Documentation | n/a | No help slot in a Telegram message. |
| Total | | 22/36 | Acceptable |

## Design Specificity Verdict
Authored, with one category-default corner: head grammar, strike-through and unlit lamp are the product's; the MR emoji-stat line and the push card are category-interchangeable. Detector: 0 findings on nine rendered card fragments (weak evidence, web-page rule set). No overlay: no served page.

## Priority Issues
- [P1] Failure reason folded under a line count; `(script failure)` uninformative. Fix: summary = first error line; drop script_failure. /impeccable clarify
- [P1] Reaction channel contradicts the text (⚡ on passed, ⚡ on failed, 👀 on merged). Fix: removed reactions entirely. /impeccable harden
- [P1] One-tap production deploy. Fix: two-step play. /impeccable harden
- [P2] Links inside summary fight expand; failed job named three times. Fix: plain summary, one log target. /impeccable distill
- [P2] Running card tallest bubble, ten open lines every 15 s. Fix: three lines, newest card only. /impeccable quieter

## Persona Red Flags
Alex: 15 tappables, error folded, reactions undiscoverable. Casey: three links per thumb width, summary tap opens Safari, card grows under thumb, truncated MR button. Sam: colour-only state circles, faint mark, strike read as plain. Dani: failed card buried above newer pushes; heading louder than status in a one-project group.

## Minor Observations
`1 approvals`; reviewers line reads as approved; merged MR keeps failed-pipeline warning; footer repeats anchor; branch-deleted redundancy; two clocks per card; `0s` failed job; ⚠️ triple duty.

## Questions to Consider
Fold summary as the error line? Closing footer without the object? Heading only when the project changes? Pipeline card absorbing the push? Honest keyboard when the bot cannot act?
