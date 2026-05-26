package image

import "testing"

func TestFilenameToLikePattern(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text matches as contains",
			input: "mexico",
			want:  "%mexico%",
		},
		{
			name:  "star expands to percent",
			input: "new*",
			want:  "new%",
		},
		{
			name:  "question expands to underscore",
			input: "photo?01*",
			want:  "photo_01%",
		},
		{
			name:  "like metacharacters are escaped",
			input: "photo_01%",
			want:  "%photo\\_01\\%%",
		},
		{
			name:  "backslashes are escaped",
			input: "folder\\name",
			want:  "%folder\\\\name%",
		},
		{
			name:  "mixed wildcard and escaped characters",
			input: "*_%",
			want:  "%\\_\\%",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := filenameToLikePattern(tc.input); got != tc.want {
				t.Fatalf("filenameToLikePattern(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
