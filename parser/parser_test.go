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
			want:     ParsedFile{Number: "SIVR-476", RawNumber: "sivr00476", Part: 3, Tags: []string{"8k"}, Ext: ".mp4", SourceSite: "twojav.com"},
		},
		{
			name:     "standard hyphenated",
			filename: "ACHJ-057.mp4",
			want:     ParsedFile{Number: "ACHJ-057", RawNumber: "achj057", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "no hyphen with leading zeros",
			filename: "hmn690.mp4",
			want:     ParsedFile{Number: "HMN-690", RawNumber: "hmn690", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "mgstage format",
			filename: "300MAAN-783.mp4",
			want:     ParsedFile{Number: "300MAAN-783", RawNumber: "300maan783", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "heyzo format",
			filename: "HEYZO-3421.mp4",
			want:     ParsedFile{Number: "HEYZO-3421", RawNumber: "heyzo3421", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "numeric prefix label with part and tag",
			filename: "4k2.com@13dsvr01801_2_8k.mp4",
			want:     ParsedFile{Number: "DSVR-1801", RawNumber: "13dsvr01801", Part: 2, Tags: []string{"8k"}, Ext: ".mp4", SourceSite: "4k2.com"},
		},
		{
			name:     "dotPartN suffix",
			filename: "13dsvr01737.part2.mp4",
			want:     ParsedFile{Number: "DSVR-1737", RawNumber: "13dsvr01737", Part: 2, Ext: ".mp4"},
		},
		{
			name:     "trailing part without second underscore",
			filename: "mdvr00336_2.mp4",
			want:     ParsedFile{Number: "MDVR-336", RawNumber: "mdvr00336", Part: 2, Ext: ".mp4"},
		},
		{
			name:     "dotPartN with site prefix",
			filename: "4k2.com@hnvr00141.part3.mp4",
			want:     ParsedFile{Number: "HNVR-141", RawNumber: "hnvr00141", Part: 3, Tags: nil, Ext: ".mp4", SourceSite: "4k2.com"},
		},
		{
			name:     "six letter prefix with trailing part",
			filename: "urvrsp-229-3.mp4",
			want:     ParsedFile{Number: "URVRSP-229", RawNumber: "urvrsp229", Part: 3, Ext: ".mp4"},
		},
		{
			name:     "hash suffix no separator",
			filename: "mdvr00271vrv18khia2.mp4",
			want:     ParsedFile{Number: "MDVR-271", RawNumber: "mdvr00271", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "hash suffix no separator 2",
			filename: "savr00304vrv1uhqf2.mp4",
			want:     ParsedFile{Number: "SAVR-304", RawNumber: "savr00304", Part: 0, Ext: ".mp4"},
		},
		{
			name:     "letter part A",
			filename: "HNVR-011-A.mp4",
			want:     ParsedFile{Number: "HNVR-011", RawNumber: "hnvr011", Part: 1, Ext: ".mp4"},
		},
		{
			name:     "letter part C lowercase",
			filename: "HNVR-011-c.mp4",
			want:     ParsedFile{Number: "HNVR-011", RawNumber: "hnvr011", Part: 3, Ext: ".mp4"},
		},
		{
			name:     "bracket site prefix with part",
			filename: "[fbfb.me]sivr00102.part2.mp4",
			want:     ParsedFile{Number: "SIVR-102", RawNumber: "sivr00102", Part: 2, Ext: ".mp4", SourceSite: "fbfb.me"},
		},
		{
			name:     "h_ prefix with numeric content ID",
			filename: "h_386acrn00119.mkv",
			want:     ParsedFile{Number: "386ACRN-119", RawNumber: "386acrn00119", Part: 0, Ext: ".mkv"},
		},
		{
			name:     "h_ prefix with numeric content ID 2",
			filename: "h_454dplt09676.mkv",
			want:     ParsedFile{Number: "454DPLT-9676", RawNumber: "454dplt09676", Part: 0, Ext: ".mkv"},
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
			if got.RawNumber != tt.want.RawNumber {
				t.Errorf("RawNumber = %q, want %q", got.RawNumber, tt.want.RawNumber)
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
