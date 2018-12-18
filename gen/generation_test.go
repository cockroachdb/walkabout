// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.

package gen

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Verify that our example data in the demo package is correct and
// that we won't break the existing test code with updated outputs.
// This test has two passes.  The first generates the code we want
// to emit and the second performs a complete type-checking of the
// demo package to make sure that any changes to the generated
// code will compile.
func TestExampleData(t *testing.T) {
	a := assert.New(t)
	outputs := make(map[string][]byte)
	g := newGenerationForTesting("../demo", []string{"Target"}, outputs)

	if !a.NoError(g.Execute()) {
		for k, v := range outputs {
			t.Logf("%s\n%s\n\n\n", k, string(v))
		}
	}

	a.Len(g.visitations, 1)
	v, ok := g.visitations["Target"]
	a.True(ok, "did not find Target interface")
	a.Equal("Target", v.Intf.String(), "wrong intfname")

	a.Len(v.Structs, 3)
	v.checkStructInfo(a, "ContainerType", byRef, 16)
	v.checkStructInfo(a, "ByValType", byValue, 0)
	v.checkStructInfo(a, "ByRefType", byRef, 0)

	v.checkVisitableInterface(a, "Target")
	v.checkVisitableInterface(a, "EmbedsTarget")

	g = newGenerationForTesting("../demo", []string{"Target"}, outputs)
	g.fullCheck = true
	g.extraTestSource = outputs
	if !a.NoError(g.Execute(), "could not parse with generated code") {
		for k, v := range outputs {
			t.Logf("%s\n%s\n\n\n", k, string(v))
		}
	}
}

// Run the generator twice to ensure that it produces stable output.
func TestOutputIsStable(t *testing.T) {
	a := assert.New(t)

	outputs1 := make(map[string][]byte)
	g1 := newGenerationForTesting("../demo", []string{"Target"}, outputs1)
	a.NoError(g1.Execute())
	a.True(len(outputs1) > 0, "no outputs")

	outputs2 := make(map[string][]byte)
	g2 := newGenerationForTesting("../demo", []string{"Target"}, outputs2)
	a.NoError(g2.Execute())
	a.True(len(outputs2) > 0, "no outputs")

	a.Equal(outputs1, outputs2)
}

func (v *visitation) checkVisitableInterface(a *assert.Assertions, name string) {
	obj := v.pkg.Scope().Lookup(name)
	if !a.NotNil(obj, "did not find", name) {
		return
	}
	vt, ok := v.visitableType(obj.Type())
	a.True(ok, name, "was not a visitableType")
	a.IsType(namedInterfaceType{}, vt, name)
}

func (v *visitation) checkStructInfo(
	a *assert.Assertions, name string, implMode refMode, fieldCount int,
) {
	s, ok := v.Structs[name]
	if !a.True(ok, "did not find", name) {
		return
	}
	a.Equal(implMode, s.implMode)
	a.Len(s.Fields(), fieldCount)
}

// newGenerationForTesting creates a generator that captures
// its output in the provided map.
func newGenerationForTesting(
	dir string, typeNames []string, outputs map[string][]byte,
) *generation {
	g := newGeneration(dir, typeNames)
	var mu sync.Mutex
	g.writeCloser = func(name string) (io.WriteCloser, error) {
		return newMapWriter(name, &mu, outputs), nil
	}
	return g
}

// mapWriter is a trivial implementation of io.WriteCloser that captures
// its output in a map. Access to the map is synchronized via a
// shared mutex.
type mapWriter struct {
	buf  bytes.Buffer
	name string
	mu   struct {
		*sync.Mutex
		dest map[string][]byte
	}
}

func newMapWriter(name string, mu *sync.Mutex, outputs map[string][]byte) io.WriteCloser {
	ret := &mapWriter{name: name}
	ret.mu.Mutex = mu
	ret.mu.dest = outputs
	return ret
}

// Write implements io.Writer.
func (w *mapWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// Close implements io.Closer.
func (w *mapWriter) Close() error {
	w.mu.Lock()
	if w.mu.dest != nil {
		w.mu.dest[w.name] = w.buf.Bytes()
	}
	w.mu.Unlock()
	return nil
}
