package semver

import (
	"errors"
	"strconv"
	"strings"
)

type Version struct {
	Major, Minor, Patch uint64
	Pre, Build          []string
}

func validIdent(s string, numericNoLeadingZero bool) bool {
	if s == "" {
		return false
	}
	allDigit := true
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
			return false
		}
		if r < '0' || r > '9' {
			allDigit = false
		}
	}
	if numericNoLeadingZero && allDigit && len(s) > 1 && s[0] == '0' {
		return false
	}
	return true
}

func num(s string) (uint64, error) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, errors.New("bad number")
	}
	return strconv.ParseUint(s, 10, 64)
}

func Parse(s string) (Version, error) {
	var v Version
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		v.Build = strings.Split(s[i+1:], ".")
		for _, id := range v.Build {
			if !validIdent(id, false) {
				return Version{}, errors.New("semver: bad build")
			}
		}
		s = s[:i]
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		v.Pre = strings.Split(s[i+1:], ".")
		for _, id := range v.Pre {
			if !validIdent(id, true) {
				return Version{}, errors.New("semver: bad pre-release")
			}
		}
		s = s[:i]
	}
	p := strings.Split(s, ".")
	if len(p) != 3 {
		return Version{}, errors.New("semver: need major.minor.patch")
	}
	var err error
	for i, dst := range []*uint64{&v.Major, &v.Minor, &v.Patch} {
		if *dst, err = num(p[i]); err != nil {
			return Version{}, errors.New("semver: bad numeric part")
		}
	}
	return v, nil
}

func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (v Version) String() string {
	s := strconv.FormatUint(v.Major, 10) + "." + strconv.FormatUint(v.Minor, 10) + "." + strconv.FormatUint(v.Patch, 10)
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	if len(v.Build) > 0 {
		s += "+" + strings.Join(v.Build, ".")
	}
	return s
}

func cmpU(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func Compare(a, b Version) int {
	if c := cmpU(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpU(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmpU(a.Patch, b.Patch); c != 0 {
		return c
	}
	if len(a.Pre) == 0 && len(b.Pre) == 0 {
		return 0
	}
	if len(a.Pre) == 0 {
		return 1
	}
	if len(b.Pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		x, y := a.Pre[i], b.Pre[i]
		xn, ex := strconv.ParseUint(x, 10, 64)
		yn, ey := strconv.ParseUint(y, 10, 64)
		switch {
		case ex == nil && ey == nil:
			if c := cmpU(xn, yn); c != 0 {
				return c
			}
		case ex == nil:
			return -1
		case ey == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return cmpU(uint64(len(a.Pre)), uint64(len(b.Pre)))
}
