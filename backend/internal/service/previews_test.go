package service

import "testing"

func TestPreviewKindAllowsOnlyBrowserSafeFormats(t *testing.T) {
	tests := []struct {
		name      string
		mimeType  string
		sizeBytes int64
		want      PreviewKind
		allowed   bool
	}{
		{name: "raster image", mimeType: "image/png", sizeBytes: 12, want: PreviewKindImage, allowed: true},
		{name: "pdf", mimeType: "application/pdf", sizeBytes: 12, want: PreviewKindPDF, allowed: true},
		{name: "video", mimeType: "video/mp4", sizeBytes: 12, want: PreviewKindVideo, allowed: true},
		{name: "audio", mimeType: "audio/mpeg", sizeBytes: 12, want: PreviewKindAudio, allowed: true},
		{name: "plain text", mimeType: "text/plain", sizeBytes: 12, want: PreviewKindText, allowed: true},
		{name: "json text", mimeType: "application/json", sizeBytes: 12, want: PreviewKindText, allowed: true},
		{name: "active svg", mimeType: "image/svg+xml", sizeBytes: 12, allowed: false},
		{name: "html", mimeType: "text/html", sizeBytes: 12, want: PreviewKindText, allowed: true},
		{name: "oversized text", mimeType: "text/plain", sizeBytes: maxTextPreviewBytes + 1, allowed: false},
		{name: "binary", mimeType: "application/octet-stream", sizeBytes: 12, allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind, allowed := previewKind(test.mimeType, test.sizeBytes)
			if allowed != test.allowed || kind != test.want {
				t.Fatalf("previewKind(%q, %d) = (%q, %t), want (%q, %t)", test.mimeType, test.sizeBytes, kind, allowed, test.want, test.allowed)
			}
		})
	}
}
