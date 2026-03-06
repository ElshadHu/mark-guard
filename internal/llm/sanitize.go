// Package llm handles communication with LLM APIs
package llm

import "regexp"

// htmlCommentRe matches HTML comments in markdown — compiled once at package level.
var htmlCommentRe = regexp.MustCompile(`<!--[\s\S]*?-->`)

// StripHTMLComments removes all HTML comments from markdown
func StripHTMLComments(md string) string {
	return htmlCommentRe.ReplaceAllString(md, "")
}

// WrapCodeDiff wraps the code diff in <CODE_CHANGES> tags
func WrapCodeDiff(diff string) string {
	return "<CODE_CHANGES>\n" + diff + "\n</CODE_CHANGES>"
}

// WrapDoc wraps the doc content in <DOCUMENT path="..."> tags
func WrapDoc(path, content string) string {
	return `<DOCUMENT path="` + path + `">` + "\n" + content + "\n</DOCUMENT>"
}

// WrapRole wraps the role in <ROLE>
func WrapRole(role string) string {
	return "<ROLE>\n" + role + "\n</ROLE>"
}

// WrapContext wraps the context in <CONTEXT>
func WrapContext(ctx string) string {
	return "<CONTEXT>\n" + ctx + "\n</CONTEXT>"
}

// WrapScale wraps the scale in <SCALE>
func WrapScale(scale string) string {
	return "<SCALE>\n" + scale + "\n</SCALE>"
}

// WrapRules wraps the rules in <RULES>
func WrapRules(rules string) string {
	return "<RULES>\n" + rules + "\n</RULES>"
}

// WrapTone wraps the tone guidelines in <TONE>
func WrapTone(tone string) string {
	return "<TONE>\n" + tone + "\n</TONE>"
}

// WrapEdgeCases wraps the edge cases in <EDGE_CASES>
func WrapEdgeCases(ec string) string {
	return "<EDGE_CASES>\n" + ec + "\n</EDGE_CASES>"
}

// WrapExamples wraps the few-shot examples in <EXAMPLES>
func WrapExamples(examples string) string {
	return "<EXAMPLES>\n" + examples + "\n</EXAMPLES>"
}
