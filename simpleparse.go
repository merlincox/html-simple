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

const (
	WIN_LBR  = "\r\n"
	UNIX_LBR = "\n"
)

var lbr = WIN_LBR

// HtmlTexter interface for creating text from HTML
type HtmlTexter interface {
	StartTag(tag string)
	SelfTag(tag string)
	EndTag(tag string)
	Text(enclosing, input string)
	String() string
}

// SetUnixLbr with argument true sets Unix-style line-breaks in output ("\n")
// with argument false sets Windows-style line-breaks in output ("\r\n", the default)
func SetUnixLbr(b bool) {
	if b {
		lbr = UNIX_LBR
	} else {
		lbr = WIN_LBR
	}
}

// HTML2Text generate text from HTML source using the built-in HtmlTexter
func HTML2Text(src string) string {

	texter := &simpleTexter{
		sb: strings.Builder{},
	}

	err := simpleParse(src, false, texter)

	if err == nil {
		return texter.String()
	}

	return ""
}

// Custom2Text generate text from HTML source using a supplied instance of HtmlTexter
// if mustTag is true, throw an error if source appears to be plain text
func Custom2Text(src string, mustTag bool, texter HtmlTexter) (string, error){

	err := simpleParse(src, false, texter)

	if err == nil {
		return texter.String(), nil
	}

	return "", err
}

type simpleTexter struct {
	sb strings.Builder
}

func (h *simpleTexter) StartTag(tag string) {
	h.sb.WriteString(strings.Repeat(lbr, openingBreakers[tag]))
}

func (h *simpleTexter) SelfTag(tag string) {
	h.sb.WriteString(strings.Repeat(lbr, selfBreakers[tag]))
}

func (h *simpleTexter) EndTag(tag string) {
	h.sb.WriteString(strings.Repeat(lbr, closingBreakers[tag]))
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
