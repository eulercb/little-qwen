package semver

import (
	"reflect"
	"testing"
)

func TestHiddenParseValid(t *testing.T) {
	v, err := Parse("v1.2.3-alpha.1+build.5")
	if err != nil {
		t.Fatal(err)
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 || !reflect.DeepEqual(v.Pre, []string{"alpha", "1"}) || !reflect.DeepEqual(v.Build, []string{"build", "5"}) {
		t.Fatalf("%+v", v)
	}
	if v.String() != "1.2.3-alpha.1+build.5" {
		t.Fatal(v.String())
	}
	for _, s := range []string{"0.0.0", "1.0.0-0A.is.legal", "1.0.0+001", "1.0.0-x-y-z.--", "10.20.30"} {
		if _, err := Parse(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
}

func TestHiddenParseInvalid(t *testing.T) {
	for _, s := range []string{"1.2", "1.2.3.4", "01.2.3", "1.02.3", "1.0.0-01", "1.0.0-alpha..1", "1.0.0-al$pha", "", "a.b.c", "1.0.0+", "1.0.0-"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("%q: expected error", s)
		}
	}
}

func TestHiddenPrecedenceChain(t *testing.T) {
	chain := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for i := 0; i+1 < len(chain); i++ {
		a, b := MustParse(chain[i]), MustParse(chain[i+1])
		if Compare(a, b) != -1 || Compare(b, a) != 1 {
			t.Errorf("%s < %s failed", chain[i], chain[i+1])
		}
	}
}

func TestHiddenBuildIgnored(t *testing.T) {
	if Compare(MustParse("1.0.0+a"), MustParse("1.0.0+b")) != 0 {
		t.Fatal("build metadata must be ignored")
	}
}

func TestHiddenMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	MustParse("nope")
}
