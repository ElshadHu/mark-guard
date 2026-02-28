// Package llm handles communication with LLM APIs
package llm

import "regexp"

// StripHTMLComments removes all comments from markdown
func StripHTMLComments(md string) string {
	re := regexp.MustCompile(`<!--[\s\S]*?-->`)
	return re.ReplaceAllString(md, "")
}

// WrapCodeDiff wraps the code diff in <CODE_CHANGES> tags
func WrapCodeDiff(diff string) string {
	return "<CODE_CHANGES>\n" + diff + "\n</CODE_CHANGES>"
}

// WrapDoc wraps the doc content in <DOCUMENT path="..."> tags
func WrapDoc(path, content string) string {
	return `<DOCUMENT path="` + path + `">` + "\n" + content + "\n</DOCUMENT>"
}
