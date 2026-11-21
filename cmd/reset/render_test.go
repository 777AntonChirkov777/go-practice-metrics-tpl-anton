package main

import (
	"go/types"
	"path"
	"testing"
)

func TestImports(t *testing.T) {
	tests := []struct {
		name    string
		modPath string
		paths   []string
		want    string
	}{
		{
			name:    "none",
			modPath: "example.com/mod",
		},
		{
			name:    "single",
			modPath: "example.com/mod",
			paths:   []string{"bytes"},
			want:    "import \"bytes\"\n",
		},
		{
			name:    "std only",
			modPath: "example.com/mod",
			paths:   []string{"io", "bytes", "encoding/json"},
			want:    "import (\n\t\"bytes\"\n\t\"encoding/json\"\n\t\"io\"\n)\n",
		},
		{
			name:    "three groups",
			modPath: "example.com/mod",
			paths: []string{
				"golang.org/x/tools/go/packages",
				"example.com/mod/z",
				"io",
				"github.com/x/util",
				"example.com/mod",
				"bytes",
				"example.com/mod/a/util",
				"github.com/a/b",
			},
			want: "import (\n" +
				"\t\"bytes\"\n" +
				"\t\"io\"\n" +
				"\n" +
				"\t\"github.com/a/b\"\n" +
				"\tutil2 \"github.com/x/util\"\n" +
				"\t\"golang.org/x/tools/go/packages\"\n" +
				"\n" +
				"\t\"example.com/mod\"\n" +
				"\t\"example.com/mod/a/util\"\n" +
				"\t\"example.com/mod/z\"\n" +
				")\n",
		},
		{
			name:    "module path without a dot",
			modPath: "practice",
			paths:   []string{"practice/internal/model", "net/http", "golang.org/x/sync/errgroup", "practice"},
			want: "import (\n" +
				"\t\"net/http\"\n" +
				"\n" +
				"\t\"golang.org/x/sync/errgroup\"\n" +
				"\n" +
				"\t\"practice\"\n" +
				"\t\"practice/internal/model\"\n" +
				")\n",
		},
		{
			name:    "std and local",
			modPath: "practice",
			paths:   []string{"practice/internal/audit", "time"},
			want:    "import (\n\t\"time\"\n\n\t\"practice/internal/audit\"\n)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := &unit{
				pkg:     types.NewPackage(tt.modPath+"/target", "target"),
				modPath: tt.modPath,
				names:   map[string]string{},
				refs:    map[string]*types.Package{},
			}
			for _, p := range tt.paths {
				u.qualify(types.NewPackage(p, path.Base(p)))
			}
			u.assignNames(nil)
			if got := u.imports(); got != tt.want {
				t.Errorf("imports() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
