package common

import (
	"bytes"
	"html/template"
)

// VerificationEmail is an email that carries a verification code or a
// confirmation link. Every field is plain text; RenderVerificationEmail
// escapes it.
type VerificationEmail struct {
	// Preview is the line inbox lists show next to the subject.
	Preview string
	Title   string
	Intro   string
	// Code is shown large and on its own so one double-click or long press
	// selects all of it.
	Code string
	// ActionURL and ActionLabel make a button. LinkHint introduces the URL
	// written out for clients where the button does not open.
	ActionURL   string
	ActionLabel string
	LinkHint    string
	// Hints are short lines under the code or button, such as how long it
	// stays valid.
	Hints []string
	// Notice closes the email, such as what to do if the reader did not ask
	// for it.
	Notice string
	Footer string
}

// Email clients drop <style> blocks and scripts, so the layout uses tables
// and inline styles only.
var verificationEmailTemplate = template.Must(template.New("verification-email").Parse(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;">
{{- if .Preview}}
<div style="display:none;max-height:0;overflow:hidden;opacity:0;">{{.Preview}}</div>
{{- end}}
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f4f4f5;">
<tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width:480px;background-color:#ffffff;border:1px solid #e4e4e7;border-radius:12px;">
<tr><td style="padding:28px 32px 0;font-family:{{template "font"}};font-size:15px;font-weight:600;color:#18181b;">{{.SystemName}}</td></tr>
<tr><td style="padding:20px 32px 0;font-family:{{template "font"}};">
<h1 style="margin:0;font-size:22px;line-height:1.4;font-weight:600;color:#18181b;">{{.Title}}</h1>
<p style="margin:12px 0 0;font-size:15px;line-height:1.7;color:#52525b;">{{.Intro}}</p>
</td></tr>
<tr><td style="padding:24px 32px 0;font-family:{{template "font"}};">
{{- if .Code}}
<div style="background-color:#f4f4f5;border-radius:8px;padding:20px 12px;text-align:center;font-family:'SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace;font-size:32px;line-height:1.2;font-weight:700;letter-spacing:8px;color:#18181b;-webkit-user-select:all;user-select:all;">{{.Code}}</div>
{{- end}}
{{- if .ActionURL}}
<table role="presentation" cellpadding="0" cellspacing="0" border="0" align="center"><tr><td style="border-radius:8px;background-color:#18181b;">
<a href="{{.ActionURL}}" target="_blank" rel="noopener" style="display:inline-block;padding:12px 28px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;">{{.ActionLabel}}</a>
</td></tr></table>
{{- end}}
{{- range .Hints}}
<p style="margin:12px 0 0;text-align:center;font-size:13px;line-height:1.6;color:#71717a;">{{.}}</p>
{{- end}}
{{- if .ActionURL}}
<p style="margin:20px 0 0;font-size:13px;line-height:1.6;color:#71717a;">{{.LinkHint}}</p>
<p style="margin:6px 0 0;font-size:13px;line-height:1.6;color:#52525b;word-break:break-all;">{{.ActionURL}}</p>
{{- end}}
</td></tr>
<tr><td style="padding:24px 32px 28px;font-family:{{template "font"}};">
<p style="margin:0;padding-top:20px;border-top:1px solid #f4f4f5;font-size:13px;line-height:1.7;color:#71717a;">{{.Notice}}</p>
</td></tr>
</table>
<p style="margin:16px 0 0;font-family:{{template "font"}};font-size:12px;line-height:1.6;color:#a1a1aa;">{{.Footer}}</p>
</td></tr>
</table>
</body>
</html>
{{define "font"}}-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',Helvetica,Arial,sans-serif{{end}}`))

// RenderVerificationEmail lays out the email as HTML under the site name.
func RenderVerificationEmail(email VerificationEmail) (string, error) {
	var out bytes.Buffer
	data := struct {
		VerificationEmail
		SystemName string
	}{email, SystemName}
	if err := verificationEmailTemplate.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}
