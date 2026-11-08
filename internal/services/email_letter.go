package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"strings"
	texttemplate "text/template"
	"time"
)

// The panel's emails are letters: one layout (layout.html) drawn as an airmail letter, filled
// with the texts of each email in the notification language (messages.<lang>.json). Every text
// is a text/template over the email's data (.COMPANY_NAME, .Server.Name…); the layout is
// html/template, so whatever a text renders (a server's name included) is escaped there.

// letterLanguages are the languages the panel writes its emails in; the first is the default.
var letterLanguages = []string{"es", "en"}

type letterCatalogFile struct {
	Common struct {
		Greeting  string `json:"greeting"`
		Stamp     string `json:"stamp"`
		Ref       string `json:"ref"`
		Fallback  string `json:"fallback"`
		Signoff   string `json:"signoff"`
		Signature string `json:"signature"`
		Footer    string `json:"footer"`
		OpenPanel string `json:"openPanel"`
	} `json:"common"`
	Emails map[string]letterTextFile `json:"emails"`
}

type letterTextFile struct {
	Subject    string   `json:"subject"`
	Preheader  string   `json:"preheader"`
	Title      string   `json:"title"`
	Paragraphs []string `json:"paragraphs"`
	Details    []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	} `json:"details"`
	Action *struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	} `json:"action"`
}

// letterText is one text of a letter, ready to render with the email's data.
type letterText struct{ tmpl *texttemplate.Template }

func compileLetterText(name, source string) (letterText, error) {
	t, err := texttemplate.New(name).Parse(source)
	if err != nil {
		return letterText{}, fmt.Errorf("email text %s: %w", name, err)
	}
	return letterText{tmpl: t}, nil
}

// render fills the text in. A field the data does not have renders as nothing, never as
// "<no value>".
func (lt letterText) render(data map[string]interface{}) (string, error) {
	if lt.tmpl == nil {
		return "", nil
	}
	var b strings.Builder
	if err := lt.tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.ReplaceAll(b.String(), "<no value>", "")), nil
}

type letterPair struct{ label, value letterText }

type letterEmail struct {
	subject, preheader, title letterText
	paragraphs                []letterText
	details                   []letterPair
	action                    *letterPair
}

type letterCommon struct {
	greeting, stamp, ref, fallback, signoff, signature, footer, openPanel letterText
}

type letterCatalog struct {
	common letterCommon
	emails map[string]*letterEmail
}

// letterBook holds the layout and the texts of every language.
type letterBook struct {
	layout   *htmltemplate.Template
	catalogs map[string]*letterCatalog
}

func loadLetterBook(fsys fs.ReadFileFS) (*letterBook, error) {
	layoutSource, err := fsys.ReadFile("layout.html")
	if err != nil {
		return nil, err
	}
	layout, err := htmltemplate.New("layout").Parse(string(layoutSource))
	if err != nil {
		return nil, fmt.Errorf("email layout: %w", err)
	}
	book := &letterBook{layout: layout, catalogs: make(map[string]*letterCatalog)}
	for _, lang := range letterLanguages {
		raw, err := fsys.ReadFile("messages." + lang + ".json")
		if err != nil {
			return nil, err
		}
		catalog, err := compileLetterCatalog(lang, raw)
		if err != nil {
			return nil, err
		}
		book.catalogs[lang] = catalog
	}
	return book, nil
}

func compileLetterCatalog(lang string, raw []byte) (*letterCatalog, error) {
	var file letterCatalogFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("messages.%s.json: %w", lang, err)
	}
	var errs []error
	text := func(name, source string) letterText {
		t, err := compileLetterText(lang+"."+name, source)
		if err != nil {
			errs = append(errs, err)
		}
		return t
	}
	c := file.Common
	catalog := &letterCatalog{
		common: letterCommon{
			greeting: text("greeting", c.Greeting), stamp: text("stamp", c.Stamp), ref: text("ref", c.Ref),
			fallback: text("fallback", c.Fallback), signoff: text("signoff", c.Signoff),
			signature: text("signature", c.Signature), footer: text("footer", c.Footer), openPanel: text("openPanel", c.OpenPanel),
		},
		emails: make(map[string]*letterEmail),
	}
	for name, e := range file.Emails {
		email := &letterEmail{
			subject:   text(name+".subject", e.Subject),
			preheader: text(name+".preheader", e.Preheader),
			title:     text(name+".title", e.Title),
		}
		for i, p := range e.Paragraphs {
			email.paragraphs = append(email.paragraphs, text(fmt.Sprintf("%s.paragraphs.%d", name, i), p))
		}
		for i, d := range e.Details {
			email.details = append(email.details, letterPair{
				label: text(fmt.Sprintf("%s.details.%d.label", name, i), d.Label),
				value: text(fmt.Sprintf("%s.details.%d.value", name, i), d.Value),
			})
		}
		if e.Action != nil {
			email.action = &letterPair{label: text(name+".action.label", e.Action.Label), value: text(name+".action.url", e.Action.URL)}
		}
		catalog.emails[name] = email
	}
	if len(errs) > 0 {
		return nil, errs[0]
	}
	return catalog, nil
}

// letterView is what layout.html draws.
type letterView struct {
	Lang, Company, Date, Ref, Preheader, Title, Greeting string
	Paragraphs                                           []string
	Details                                              []letterDetail
	Action                                               *letterAction
	Stamp, RefLabel, Fallback, Signoff, Signature        string
	Footer, OpenPanel, PanelURL                          string
	// a fixed 600px table for Outlook for Windows, which ignores max-width; html/template
	// drops the comments it hides in, so they come in as trusted HTML
	MsoOpen, MsoClose htmltemplate.HTML
}

const (
	msoOpen  = htmltemplate.HTML(`<!--[if mso]><table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" align="center"><tr><td><![endif]-->`)
	msoClose = htmltemplate.HTML(`<!--[if mso]></td></tr></table><![endif]-->`)
)

type letterDetail struct{ Label, Value string }
type letterAction struct{ Label, URL string }

// has reports whether the language has the email.
func (b *letterBook) has(lang, name string) bool {
	c := b.catalogs[lang]
	return c != nil && c.emails[name] != nil
}

// render writes the email's subject and HTML body in the language.
func (b *letterBook) render(lang, name string, data map[string]interface{}) (subject, body string, err error) {
	catalog := b.catalogs[lang]
	if catalog == nil {
		catalog = b.catalogs[letterLanguages[0]]
	}
	email := catalog.emails[name]
	if email == nil {
		return "", "", fmt.Errorf("no email %s in messages.%s.json", name, lang)
	}
	r := func(t letterText) string {
		if err != nil {
			return ""
		}
		var s string
		s, err = t.render(data)
		return s
	}
	view := letterView{
		Lang:      lang,
		Company:   fmt.Sprint(data["COMPANY_NAME"]),
		Date:      fmt.Sprint(data["DATE"]),
		Ref:       fmt.Sprint(data["REF"]),
		Preheader: r(email.preheader),
		Title:     r(email.title),
		Greeting:  r(catalog.common.greeting),
		Stamp:     r(catalog.common.stamp),
		RefLabel:  r(catalog.common.ref),
		Fallback:  r(catalog.common.fallback),
		Signoff:   r(catalog.common.signoff),
		Signature: r(catalog.common.signature),
		Footer:    r(catalog.common.footer),
		OpenPanel: r(catalog.common.openPanel),
		PanelURL:  fmt.Sprint(data["BASE_URL"]),
		MsoOpen:   msoOpen,
		MsoClose:  msoClose,
	}
	// a paragraph, detail or button that renders empty (a branch that does not apply, a value
	// the email does not have) is left out
	for _, p := range email.paragraphs {
		if s := r(p); s != "" {
			view.Paragraphs = append(view.Paragraphs, s)
		}
	}
	for _, d := range email.details {
		if label, value := r(d.label), r(d.value); label != "" && value != "" {
			view.Details = append(view.Details, letterDetail{Label: label, Value: value})
		}
	}
	if email.action != nil {
		if label, url := r(email.action.label), r(email.action.value); label != "" && url != "" {
			view.Action = &letterAction{Label: label, URL: url}
		}
	}
	subject = oneLine(r(email.subject))
	if err != nil {
		return "", "", err
	}
	var out strings.Builder
	if err = b.layout.Execute(&out, view); err != nil {
		return "", "", err
	}
	return subject, out.String(), nil
}

// oneLine keeps a subject on one line: a server's name is chosen by its owner, and a line
// break in a header would start another header.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// emailRefAlphabet leaves out the letters and digits that read alike (0/O, 1/I/L).
const emailRefAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// newEmailRef is the reference printed on an email's stamp and written to the log, so a
// person can quote it and the log tells which email it was: XXXX-XXXX.
func newEmailRef() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf) // never fails since Go 1.24
	out := make([]byte, 0, 9)
	for i, c := range buf {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, emailRefAlphabet[int(c)%len(emailRefAlphabet)])
	}
	return string(out)
}

var monthsES = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// letterDate writes the date the way a letter in the language does.
func letterDate(lang string, t time.Time) string {
	if lang == "en" {
		return t.Format("2 January 2006")
	}
	return fmt.Sprintf("%d de %s de %d", t.Day(), monthsES[t.Month()-1], t.Year())
}

// letterTime is the time of day with its zone, so a reader elsewhere can place it.
func letterTime(t time.Time) string {
	return t.Format("15:04 MST")
}
