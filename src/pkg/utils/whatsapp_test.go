package utils

import "testing"

func TestDetermineMediaExtension(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		mimeType   string
		wantSuffix string
	}{
		{
			name:       "DocxFromFilename",
			filename:   "report.docx",
			mimeType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			wantSuffix: ".docx",
		},
		{
			name:       "XlsxFromMime",
			filename:   "",
			mimeType:   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			wantSuffix: ".xlsx",
		},
		{
			name:       "PptxFromMime",
			filename:   "",
			mimeType:   "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			wantSuffix: ".pptx",
		},
		{
			name:       "ZipFallback",
			filename:   "",
			mimeType:   "application/zip",
			wantSuffix: ".zip",
		},
		{
			name:       "ExeFromFilename",
			filename:   "installer.exe",
			mimeType:   "application/octet-stream",
			wantSuffix: ".exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineMediaExtension(tt.filename, tt.mimeType)
			if got != tt.wantSuffix {
				t.Fatalf("determineMediaExtension() = %q, want %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestMatchWhatsAppIdentities(t *testing.T) {
	tests := []struct {
		name string
		id1  string
		id2  string
		want bool
	}{
		{
			name: "exact match normal phone",
			id1:  "51987654321",
			id2:  "51987654321",
			want: true,
		},
		{
			name: "match with whatsapp net suffix",
			id1:  "51987654321@s.whatsapp.net",
			id2:  "51987654321",
			want: true,
		},
		{
			name: "match both with whatsapp net suffix",
			id1:  "51987654321@s.whatsapp.net",
			id2:  "51987654321@s.whatsapp.net",
			want: true,
		},
		{
			name: "match with lid suffix",
			id1:  "123456789@lid",
			id2:  "123456789",
			want: true,
		},
		{
			name: "mismatch different numbers",
			id1:  "51999999999",
			id2:  "51888888888",
			want: false,
		},
		{
			name: "one empty",
			id1:  "",
			id2:  "51987654321",
			want: false,
		},
		{
			name: "both empty",
			id1:  "",
			id2:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchWhatsAppIdentities(tt.id1, tt.id2)
			if got != tt.want {
				t.Fatalf("MatchWhatsAppIdentities(%q, %q) = %v; want %v", tt.id1, tt.id2, got, tt.want)
			}
		})
	}
}
