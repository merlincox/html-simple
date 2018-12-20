package simpleparse

import (
	"strings"
	"errors"
	"io"
	"fmt"
	"regexp"

	"golang.org/x/net/html"
	"github.com/golang-collections/collections/stack"
)

var (
	voidElements = map[string]bool{
		"area":    true,
		"base":    true,
		"br":      true,
		"col":     true,
		"command": true,
		"embed":   true,
		"hr":      true,
		"img":     true,
		"input":   true,
		"keygen":  true,
		"link":    true,
		"meta":    true,
		"param":   true,
		"source":  true,
		"track":   true,
		"wbr":     true,
	}

	selfBreakers = map[string]int{
		"br": 1,
	}

	openingBreakers = map[string]int{
		"p":  1,
		"h1": 2,
		"h2": 2,
		"h3": 2,
		"h4": 2,
		"h5": 2,
		"h6": 2,
		"ul": 1,
	}

	closingBreakers = map[string]int{
		"p":  1,
		"h1": 2,
		"h2": 2,
		"h3": 2,
		"h4": 2,
		"h5": 2,
		"h6": 2,
		"li": 1,
	}

	whiteSpaceRE = regexp.MustCompile(`[\s\p{Zs}]+`)
)

// HtmlTexter interface for creating text from HTML
type HtmlTexter interface {
	// StartTag handles a start tag
	StartTag(tag string)
	// SelfTag handles a self-closing tag
	SelfTag(tag string)
	// EndTag handles a closing tag
	EndTag(tag string)
	// Text handles a text element within an enclosing tag
	Text(enclosing, input string)
	// String returns the text
	String() string
}

// HTML2Text generates text from HTML source using the built-in HtmlTexter.
// If optional winLbr argument is set to true, linebreaks in output will follow Winsdows style ("\r\n")
func HTML2Text(src string, winLbr ...bool) string {

	lbr := "\n"

	if len(winLbr) > 0 && winLbr[0] == true {
		lbr = "\r\n"
	}

	texter := &simpleTexter{
		sb: strings.Builder{},
		lbr: lbr,
	}

	err := simpleParse(src, false, texter)

	if err == nil {
		return texter.String()
	}

	return ""
}

// Custom2Text generates text from HTML source using a supplied instance of HtmlTexter.
// If mustTag is true, it throws an error if the source appears to be plain text.
func Custom2Text(src string, mustTag bool, texter HtmlTexter) (string, error){

	err := simpleParse(src, false, texter)

	if err == nil {
		return texter.String(), nil
	}

	return "", err
}

// IsPlainText returns true if the source appears to be plain text:
// that is, it contains no tags, docType section or HTML comments
func IsPlainText(src string) bool {
	err := simpleParse(src, true, nil)

	return err != nil && err.Error() == "Contains no tags"
}

type simpleTexter struct {
	sb strings.Builder
	lbr string
}

func (h *simpleTexter) StartTag(tag string) {
	h.sb.WriteString(strings.Repeat(h.lbr, openingBreakers[tag]))
}

func (h *simpleTexter) SelfTag(tag string) {
	h.sb.WriteString(strings.Repeat(h.lbr, selfBreakers[tag]))
}

func (h *simpleTexter) EndTag(tag string) {
	h.sb.WriteString(strings.Repeat(h.lbr, closingBreakers[tag]))
}

func (h *simpleTexter) Text(enclosing, input string) {
	if enclosing != "title" {
		h.sb.WriteString(cleanWhiteSpace(input))
	}
}

func (h *simpleTexter) String() string {
	return strings.Trim(h.sb.String(), " \r\n")
}

func cleanWhiteSpace(src string) string {

	return whiteSpaceRE.ReplaceAllString(src, " ")
}

func simpleParse(src string, mustTag bool, texter HtmlTexter) error {

	tokenizer := html.NewTokenizer(strings.NewReader(src))
	tagStack := stack.New()
	untagged := true

	for {
		tokenType := tokenizer.Next()
		tn, _ := tokenizer.TagName()
		tag := string(tn)

		if tokenType == html.StartTagToken && voidElements[tag] {
			tokenType = html.SelfClosingTagToken
		}

		switch tokenType {

		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				if tagStack.Len() != 0 {
					popped := tagStack.Pop().(string)
					return fmt.Errorf("Unterminated tag: <%v>", popped)
				}
				if untagged && mustTag {
					return errors.New("Contains no tags")
				}
				return nil
			}
			return err

		case html.StartTagToken:
			untagged = false
			tagStack.Push(tag)
			if texter != nil {
				texter.StartTag(tag)
			}

		case html.TextToken:
			if texter != nil {
				enclosing := ""
				if tagStack.Len() > 0 {
					enclosing = tagStack.Peek().(string)
				}
				texter.Text(enclosing, string(tokenizer.Text()))
			}

		case html.SelfClosingTagToken, html.DoctypeToken, html.CommentToken:
			untagged = false
			if texter != nil {
				texter.SelfTag(tag)
			}

		case html.EndTagToken:
			if tagStack.Len() == 0 {
				return fmt.Errorf("End tag without start: </%v>", tag)
			}
			popped := tagStack.Pop().(string)
			if popped != tag {
				return fmt.Errorf("Tag mismatch: <%v> with </%v>", popped, tag)
			}
			if texter != nil {
				texter.EndTag(tag)
			}
		}
	}
}
