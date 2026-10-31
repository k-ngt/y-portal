package main

import "testing"

func TestInjectSrcset(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "goldmark image output",
			in:   `<p><img src="img@2x.png" alt="a"></p>`,
			want: `<p><img src="img@2x.png" srcset="img@2x.png 2x" alt="a"></p>`,
		},
		{
			name: "attributes before src are kept",
			in:   `<img class="x" id="y" src="a@2x.png">`,
			want: `<img class="x" id="y" src="a@2x.png" srcset="a@2x.png 2x">`,
		},
		{
			name: "fractional scale",
			in:   `<img src="photo@1.5x.jpg">`,
			want: `<img src="photo@1.5x.jpg" srcset="photo@1.5x.jpg 1.5x">`,
		},
		{
			name: "single quotes",
			in:   `<img alt='b' src='dir/p@3x.webp'>`,
			want: `<img alt='b' src='dir/p@3x.webp' srcset='dir/p@3x.webp 3x'>`,
		},
		{
			name: "self-closing tag",
			in:   `<img src="i@2x.png" />`,
			want: `<img src="i@2x.png" srcset="i@2x.png 2x" />`,
		},
		{
			name: "non-retina image is untouched",
			in:   `<img class="x" src="plain.png">`,
			want: `<img class="x" src="plain.png">`,
		},
		{
			name: "multiple images",
			in:   `<img src="a@2x.png"><img src="b.png"><img class="c" src="d@2x.png">`,
			want: `<img src="a@2x.png" srcset="a@2x.png 2x"><img src="b.png"><img class="c" src="d@2x.png" srcset="d@2x.png 2x">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := injectSrcset(tt.in); got != tt.want {
				t.Errorf("injectSrcset(%q)\n got: %q\nwant: %q", tt.in, got, tt.want)
			}
		})
	}
}
