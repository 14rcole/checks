// Copyright 2016 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build ignore

// mkstdlib generates the zstdlib.go file, containing the Go standard
// library API symbols. It's baked into the binary to avoid scanning
// GOPATH in the common case.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func mustOpen(name string) io.Reader {
	f, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func api(base string) string {
	return filepath.Join(os.Getenv("GOROOT"), "api", base)
}

var sym = regexp.MustCompile(`^pkg (\S+).*?, (?:var|func|type|const) ([A-Z]\w*)`)

func main() {
	var buf bytes.Buffer
	outf := func(format string, args ...interface{}) {
		fmt.Fprintf(&buf, format, args...)
	}
	outf("// AUTO-GENERATED BY mkstdlib.go\n\n")
	outf("package ptimports\n")
	outf("var stdlib = map[string]string{\n")
	f := io.MultiReader(
		mustOpen(api("go1.txt")),
		mustOpen(api("go1.1.txt")),
		mustOpen(api("go1.2.txt")),
		mustOpen(api("go1.3.txt")),
		mustOpen(api("go1.4.txt")),
		mustOpen(api("go1.5.txt")),
		mustOpen(api("go1.6.txt")),
	)
	sc := bufio.NewScanner(f)
	fullImport := map[string]string{} // "zip.NewReader" => "archive/zip"
	ambiguous := map[string]bool{}
	var keys []string
	for sc.Scan() {
		l := sc.Text()
		has := func(v string) bool { return strings.Contains(l, v) }
		if has("struct, ") || has("interface, ") || has(", method (") {
			continue
		}
		if m := sym.FindStringSubmatch(l); m != nil {
			full := m[1]
			key := path.Base(full) + "." + m[2]
			if exist, ok := fullImport[key]; ok {
				if exist != full {
					ambiguous[key] = true
				}
			} else {
				fullImport[key] = full
				keys = append(keys, key)
			}
		}
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if ambiguous[key] {
			outf("\t// %q is ambiguous\n", key)
		} else {
			outf("\t%q: %q,\n", key, fullImport[key])
		}
	}
	outf("\n")
	for _, sym := range [...]string{"Alignof", "ArbitraryType", "Offsetof", "Pointer", "Sizeof"} {
		outf("\t%q: %q,\n", "unsafe."+sym, "unsafe")
	}
	outf("}\n")
	fmtbuf, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile("zstdlib.go", fmtbuf, 0666)
	if err != nil {
		log.Fatal(err)
	}
}
