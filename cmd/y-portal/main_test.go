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
			name: "existing srcset after src is kept as is",
			in:   `<img src="a@2x.png" srcset="a.png 1x, a@2x.png 2x, a@3x.png 3x">`,
			want: `<img src="a@2x.png" srcset="a.png 1x, a@2x.png 2x, a@3x.png 3x">`,
		},
		{
			name: "existing srcset before src is kept as is",
			in:   `<img srcset="a@2x.png 2x" src="a@2x.png">`,
			want: `<img srcset="a@2x.png 2x" src="a@2x.png">`,
		},
		{
			name: "data-srcset does not count as srcset",
			in:   `<img data-srcset="x.png 1x" src="a@2x.png">`,
			want: `<img data-srcset="x.png 1x" src="a@2x.png" srcset="a@2x.png 2x">`,
		},
		{
			name: "mismatched quotes are not treated as a src value",
			in:   `<img src="a@2x.png' alt="x">`,
			want: `<img src="a@2x.png' alt="x">`,
		},
		{
			name: "apostrophe inside a double-quoted src",
			in:   `<img src="it's@2x.png">`,
			want: `<img src="it's@2x.png" srcset="it's@2x.png 2x">`,
		},
		{
			name: "double quote inside a single-quoted src",
			in:   `<img src='say"hi"@2x.png'>`,
			want: `<img src='say"hi"@2x.png' srcset='say"hi"@2x.png 2x'>`,
		},
		{
			name: "data-src before a plain src is ignored",
			in:   `<img data-src="big@2x.png" src="placeholder.png">`,
			want: `<img data-src="big@2x.png" src="placeholder.png">`,
		},
		{
			name: "data-src after a plain src is ignored",
			in:   `<img src="placeholder.png" data-src="big@2x.png">`,
			want: `<img src="placeholder.png" data-src="big@2x.png">`,
		},
		{
			name: "plain src after data-src still gets srcset",
			in:   `<img data-src="x.png" src="a@2x.png">`,
			want: `<img data-src="x.png" src="a@2x.png" srcset="a@2x.png 2x">`,
		},
		{
			name: "newline before src",
			in:   "<img\n  class=\"x\"\n  src=\"a@2x.png\">",
			want: "<img\n  class=\"x\"\n  src=\"a@2x.png\" srcset=\"a@2x.png 2x\">",
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
