package rewrite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExactRewriter(t *testing.T) {
	rw := NewExactRewriter(map[string]string{
		"github.com/foo/bar": "github.com/foo/other",
		"github.com/foo/baz": "github.com/foo/somethingelse",
	})

	input := `package example
	
import (
	_ "github.com/foo/bar"
	_ "github.com/foo/baz"
	_ "github.com/foo/third"
)
`
	expected := `package example

import (
	_ "github.com/foo/other"
	_ "github.com/foo/somethingelse"
	_ "github.com/foo/third"
)
`

	rewritten, output, err := rw.RewriteImports("example.go", input)
	require.NoError(t, err)
	require.True(t, rewritten)
	require.Equal(t, expected, output)
}

func TestPrefixRewriter(t *testing.T) {
	rw := NewPrefixRewriter(map[string]string{
		"github.com/foo/":    "github.com/bar/",
		"github.com/foo/bar": "github.com/baz/other",
	})

	input := `package example

import (
	_ "github.com/foo/baz/other"	
	_ "github.com/foo/bar/baz"
	_ "github.com/other/something"
)
`

	expected := `package example

import (
	_ "github.com/bar/baz/other"
	_ "github.com/baz/other/baz"
	_ "github.com/other/something"
)
`

	rewritten, output, err := rw.RewriteImports("example.go", input)
	require.NoError(t, err)
	require.True(t, rewritten)
	require.Equal(t, expected, output)
}
