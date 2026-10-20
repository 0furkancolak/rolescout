// Package cv reads the text of an uploaded CV. PDF, Word (.docx) and plain
// text files are understood; nothing is sent anywhere.
package cv

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// MaxFileBytes is the largest file accepted.
const MaxFileBytes = 5 << 20

// MaxTextRunes bounds the stored text.
const MaxTextRunes = 60000

var (
	// ErrUnsupported means the file type is not one of PDF, DOCX, TXT or MD.
	ErrUnsupported = errors.New("cv: unsupported file type")
	// ErrNoText means the file holds no readable text (a scan, for example).
	ErrNoText = errors.New("cv: no readable text in the file")
)

// Extract returns the text of a CV, chosen by the file's extension and checked
// against its content.
func Extract(filename string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrNoText
	}
	var text string
	var err error
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		text, err = fromPDF(data)
	case ".docx":
		text, err = fromDOCX(data)
	case ".txt", ".md", ".markdown", ".text":
		if !utf8.Valid(data) {
			return "", fmt.Errorf("%w: the text file is not UTF-8", ErrUnsupported)
		}
		text = string(data)
	default:
		return "", ErrUnsupported
	}
	if err != nil {
		return "", err
	}
	text = tidy(text)
	if utf8.RuneCountInString(text) < 40 {
		return "", ErrNoText
	}
	if r := []rune(text); len(r) > MaxTextRunes {
		text = string(r[:MaxTextRunes])
	}
	return text, nil
}

func fromPDF(data []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cv: the PDF could not be read")
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("cv: the PDF could not be read")
	}
	plain, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("cv: the PDF could not be read")
	}
	b, err := io.ReadAll(io.LimitReader(plain, MaxFileBytes))
	if err != nil {
		return "", fmt.Errorf("cv: the PDF could not be read")
	}
	return string(b), nil
}

// fromDOCX reads word/document.xml, keeping paragraph and line breaks.
func fromDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("cv: the Word file could not be read")
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("cv: the Word file could not be read")
		}
		defer rc.Close()
		dec := xml.NewDecoder(io.LimitReader(rc, 20<<20))
		var b strings.Builder
		inText := false
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("cv: the Word file could not be read")
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "t":
					inText = true
				case "tab":
					b.WriteByte(' ')
				case "br":
					b.WriteByte('\n')
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "t":
					inText = false
				case "p":
					b.WriteByte('\n')
				}
			case xml.CharData:
				if inText {
					b.Write(t)
				}
			}
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("cv: the Word file could not be read")
}

// tidy trims lines, drops control characters and collapses runs of blank lines.
func tidy(s string) string {
	s = strings.ToValidUTF8(s, "")
	var out []string
	blank := 0
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", "\n"), "\n") {
		line = strings.Map(func(r rune) rune {
			if r == '\t' {
				return ' '
			}
			if r < 32 || r == 0x7f {
				return -1
			}
			return r
		}, line)
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if blank++; blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
