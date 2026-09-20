package disclosure

import (
	"bytes"
	"html/template"
)

type ReportEvidence struct {
	Title          string
	Filename       string
	SHA256         string
	UploadedAt     string
	Summary        string
	HasSummary     bool
	RedactionNotes []string
	IncludedAs     string // filename inside the package's evidence/ folder, if included
}

type ReportTimelineEntry struct {
	Date          string
	Description   string
	EvidenceTitle string
}

type ReportData struct {
	Title             string
	GeneratedAt       string
	Evidence          []ReportEvidence
	Timeline          []ReportTimelineEntry
	IncludeSummary    bool
	IncludeTimeline   bool
	IncludePhotos     bool
	Protections       []string
	FaceBlurRequested bool
}

var reportTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{.Title}} — Shield Safe Disclosure Package</title>
<style>
  body { font-family: system-ui, -apple-system, sans-serif; max-width: 760px; margin: 40px auto; padding: 0 20px; color: #16171d; line-height: 1.5; }
  h1 { font-size: 22px; margin-bottom: 4px; }
  h2 { font-size: 16px; margin-top: 36px; border-bottom: 1px solid #e3ecf4; padding-bottom: 6px; }
  .meta { color: #5b8db8; font-size: 13px; margin-bottom: 24px; }
  .notice { background: #f4f8fb; border: 1px solid #c2d7e8; border-radius: 6px; padding: 12px 16px; font-size: 13px; margin: 16px 0; }
  .evidence-item { border: 1px solid #e3ecf4; border-radius: 6px; padding: 14px; margin-bottom: 12px; }
  .evidence-item h3 { margin: 0 0 6px; font-size: 14px; }
  .hash { font-family: ui-monospace, monospace; font-size: 12px; word-break: break-all; color: #5b8db8; }
  .redaction-notes { font-size: 13px; margin-top: 8px; }
  .redaction-notes li { margin-bottom: 2px; }
  .timeline-entry { display: flex; gap: 12px; font-size: 14px; margin-bottom: 6px; }
  .timeline-date { font-family: ui-monospace, monospace; font-size: 12px; color: #5b8db8; width: 90px; flex-shrink: 0; }
  ul { padding-left: 20px; }
</style>
</head>
<body>
  <h1>{{.Title}}</h1>
  <p class="meta">Shield Safe Disclosure Package — generated {{.GeneratedAt}}</p>

  <div class="notice">
    Shield does not determine whether an incident occurred, prove authenticity, or make legal
    or investigative conclusions. This package is a privacy-reviewed export of evidence the
    sender chose to share; SHA-256 hashes below show only whether a stored file changed after
    upload, not that any evidence is authentic.
  </div>

  {{if .Protections}}
  <h2>Privacy protections applied</h2>
  <ul>
    {{range .Protections}}<li>{{.}}</li>{{end}}
    {{if .FaceBlurRequested}}<li>Face blurring was requested but is not available in this version of Shield — no faces were blurred. Review images manually before sharing if this matters.</li>{{end}}
  </ul>
  {{else if .FaceBlurRequested}}
  <h2>Privacy protections applied</h2>
  <ul>
    <li>Face blurring was requested but is not available in this version of Shield — no faces were blurred. Review images manually before sharing if this matters.</li>
  </ul>
  {{end}}

  <h2>Evidence index</h2>
  {{range .Evidence}}
  <div class="evidence-item">
    <h3>{{.Title}}</h3>
    <p>Filename: {{.Filename}}<br>Uploaded: {{.UploadedAt}}</p>
    <p class="hash">SHA-256 (original): {{.SHA256}}</p>
    {{if .IncludedAs}}<p>Included in this package as: <code>{{.IncludedAs}}</code></p>{{end}}
    {{if .RedactionNotes}}
    <div class="redaction-notes">
      <strong>Redaction report:</strong>
      <ul>{{range .RedactionNotes}}<li>{{.}}</li>{{end}}</ul>
    </div>
    {{end}}
  </div>
  {{end}}

  {{if .IncludeSummary}}
  <h2>Incident summary</h2>
  {{range .Evidence}}
    {{if .HasSummary}}
    <p><strong>{{.Title}}:</strong> {{.Summary}}</p>
    {{end}}
  {{end}}
  {{end}}

  {{if .IncludeTimeline}}
  <h2>Evidence timeline</h2>
  {{if .Timeline}}
    {{range .Timeline}}
    <div class="timeline-entry">
      <span class="timeline-date">{{.Date}}</span>
      <span>{{.Description}} <em>({{.EvidenceTitle}})</em></span>
    </div>
    {{end}}
  {{else}}
    <p>No timeline events available for the included evidence.</p>
  {{end}}
  {{end}}
</body>
</html>
`))

func renderReport(data ReportData) ([]byte, error) {
	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
