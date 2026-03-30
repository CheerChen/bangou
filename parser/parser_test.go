package parser

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     ParsedFile
	}{
		{
			name:     "standard with site prefix and part",
			filename: "twojav.com@sivr00476_3_8k.mp4",
			want:     ParsedFile{Number: "SIVR-476", Part: 3, Tags: []string{"8k"}, Ext: ".mp4", SourceSite: "twojav.com"},
		},
		{
			name:     "standard hyphenated",
			filename: "ACHJ-057.mp4",
			want:     ParsedFile{Number: "ACHJ-057", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "no hyphen with leading zeros",
			filename: "hmn690.mp4",
			want:     ParsedFile{Number: "HMN-690", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "mgstage format",
			filename: "300MAAN-783.mp4",
			want:     ParsedFile{Number: "300MAAN-783", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "heyzo format",
			filename: "HEYZO-3421.mp4",
			want:     ParsedFile{Number: "HEYZO-3421", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "unrecognizable",
			filename: "random_video_2025.mp4",
			want:     ParsedFile{Ext: ".mp4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.Number != tt.want.Number {
				t.Errorf("Number = %q, want %q", got.Number, tt.want.Number)
			}
			if got.Part != tt.want.Part {
				t.Errorf("Part = %d, want %d", got.Part, tt.want.Part)
			}
			if got.Ext != tt.want.Ext {
				t.Errorf("Ext = %q, want %q", got.Ext, tt.want.Ext)
			}
			if got.SourceSite != tt.want.SourceSite {
				t.Errorf("SourceSite = %q, want %q", got.SourceSite, tt.want.SourceSite)
			}
		})
	}
}
