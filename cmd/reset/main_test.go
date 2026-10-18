package main

import (
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMarked(t *testing.T) {
	for _, text := range []string{"// generate:reset", "//generate:reset", "// generate:resettable", "// generate:reset extra"} {
		t.Run(text, func(t *testing.T) {
			want := text != "// generate:resettable" && text != "// generate:reset extra"
			if got := marked(&ast.CommentGroup{List: []*ast.Comment{{Text: text}}}); got != want {
				t.Fatalf("marked(%q) = %v, want %v", text, got, want)
			}
		})
	}
	if marked(nil) {
		t.Fatal("nil comment must not match")
	}
}

func TestGenerate(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module resetfixture\n\ngo 1.26.0\n")
	writeFixture(t, root, "model.go", `package fixture
import "resetfixture/nested"
type Integer int
type Slice []string
type Mapping map[string]int
type Alias = Slice
type Plain struct { Value int }
type Manual struct { Calls int }
func (m *Manual) Reset() { m.Calls++ }
// generate:reset
type Example struct {
 i, j Integer
 text string
 flag bool
 f float64
 c complex128
 p *string
 pp **int
 s Alias
 m Mapping
 ps *[]int
 pm *map[string]int
 child *Example
 value nested.Child
 manual Manual
 plain Plain
 arr [2]int
 ch chan int
 fn func()
 iface any
 _ int
}
type (
 // generate:reset
 Other struct { Value int }
 Unmarked struct { Value int }
)
// generate:reset
type Generic[T any] struct { Items []T; Values map[string]T }
`)
	writeFixture(t, root, "nested/model.go", `package nested
// generate:reset
type Child struct { Value int }
`)
	writeFixture(t, root, "ignored/model.go", "package ignored\ntype Unmarked struct { Value int }\n")
	if err := generate(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "reset.gen.go")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(first), "*Unmarked") {
		t.Fatal("generated Reset for unmarked structure")
	}
	if _, err := os.Stat(filepath.Join(root, "ignored", "reset.gen.go")); !os.IsNotExist(err) {
		t.Fatalf("unexpected output for unmarked package: %v", err)
	}
	if err := generate(root); err != nil {
		t.Fatalf("second generation: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil || string(first) != string(second) {
		t.Fatalf("generation is not deterministic: %v", err)
	}
	writeFixture(t, root, "model_test.go", `package fixture
import (
 "testing"
 "resetfixture/nested"
)
func TestReset(t *testing.T) {
 text, number := "value", 42
 ptr := &number
 slice := []int{1, 2}
 mapping := map[string]int{"key": 1}
 child := &Example{i: 12}
 x := Example{i: 1, j: 2, text: "value", flag: true, f: 1.5, c: 2i,
  p: &text, pp: &ptr, s: Alias{"a", "b"}, m: Mapping{"key": 3},
  ps: &slice, pm: &mapping, child: child, value: nested.Child{Value: 7},
  plain: Plain{Value: 9}, arr: [2]int{1, 2}, ch: make(chan int), fn: func(){}, iface: 4}
 backing, capacity := &x.s[0], cap(x.s)
 originalMap := x.m
 x.Reset()
 if x.i != 0 || x.j != 0 || x.text != "" || x.flag || x.f != 0 || x.c != 0 { t.Fatal("primitives not reset") }
 if x.p != &text || text != "" || x.pp != &ptr || ptr != &number || number != 0 { t.Fatal("pointers not retained/reset") }
 if x.s == nil || len(x.s) != 0 || cap(x.s) != capacity || &x.s[:cap(x.s)][0] != backing { t.Fatal("slice storage not retained") }
 if x.m == nil || len(x.m) != 0 || len(originalMap) != 0 { t.Fatal("map not cleared") }
 x.m["new"] = 2
 if originalMap["new"] != 2 { t.Fatal("map was replaced") }
 if x.ps != &slice || slice == nil || len(slice) != 0 || x.pm != &mapping || mapping == nil || len(mapping) != 0 { t.Fatal("pointer collections not reset") }
 if x.child != child || child.i != 0 || x.value.Value != 0 || x.manual.Calls != 1 || x.plain.Value != 9 { t.Fatal("nested structs not handled correctly") }
 if x.arr != [2]int{} || x.ch != nil || x.fn != nil || x.iface != nil { t.Fatal("other fields not reset") }
 x.Reset()
 var empty Example
 empty.Reset()
 if empty.s != nil || empty.m != nil || empty.p != nil || empty.child != nil { t.Fatal("nil fields changed") }
 var nilReceiver *Example
 nilReceiver.Reset()
 other := Other{Value: 5}
 other.Reset()
 if other.Value != 0 { t.Fatal("group declaration not reset") }
 generic := Generic[int]{Items: []int{1}, Values: map[string]int{"a": 2}}
 generic.Reset()
 if len(generic.Items) != 0 || len(generic.Values) != 0 { t.Fatal("generic collections not reset") }
}
`)
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code test: %v\n%s", err, out)
	}
	// Removing the annotation also removes this generator's stale output.
	writeFixture(t, root, "nested/model.go", "package nested\ntype Child struct { Value int }\n")
	if err := generate(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "nested", "reset.gen.go")); !os.IsNotExist(err) {
		t.Fatalf("stale output still exists: %v", err)
	}
}

func TestGenerateErrors(t *testing.T) {
	for _, tc := range []struct {
		name, source, existing, want string
	}{
		{"manual method", "// generate:reset\ntype S struct{}\nfunc (*S) Reset() {}", "", "already declares Reset"},
		{"protected file", "// generate:reset\ntype S struct{}", "package fixture\n", "refusing to overwrite"},
		{"type parameter", "// generate:reset\ntype S[T any] struct { Value T }", "", "type parameter fields are not supported"},
		{"invalid source", "type S struct { Value Missing }", "", "load resetfixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, root, "go.mod", "module resetfixture\n\ngo 1.26.0\n")
			writeFixture(t, root, "model.go", "package fixture\n"+tc.source+"\n")
			if tc.existing != "" {
				writeFixture(t, root, "reset.gen.go", tc.existing)
			}
			if err := generate(root); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("generate() = %v, want error containing %q", err, tc.want)
			}
			if tc.existing != "" {
				data, err := os.ReadFile(filepath.Join(root, "reset.gen.go"))
				if err != nil || string(data) != tc.existing {
					t.Fatalf("existing file modified: %v", err)
				}
			}
		})
	}
}
