package standard

import "testing"

// Test vectors lifted from PHP 8.5's own crypt_*.phpt files. They match
// Ulrich Drepper's reference implementation.

func TestCryptSHA256(t *testing.T) {
	cases := []struct{ salt, password, want string }{
		{"$5$saltstring", "Hello world!",
			"$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5"},
		{"$5$rounds=10000$saltstringsaltstring", "Hello world!",
			"$5$rounds=10000$saltstringsaltst$3xv.VbSHBb41AL9AvLeujZkZRBAwqFMz2.opqey6IcA"},
		{"$5$rounds=5000$toolongsaltstring", "This is just a test",
			"$5$rounds=5000$toolongsaltstrin$Un/5jzAHMgOGZ5.mWJpuVolil07guHPvOW8mGRcvxa5"},
		{"$5$rounds=1400$anotherlongsaltstring",
			"a very much longer text to encrypt.  This one even stretches over morethan one line.",
			"$5$rounds=1400$anotherlongsalts$Rx.j8H.h8HjEDGomFU8bDkXm3XIUnzyxf12oP84Bnq1"},
		{"$5$rounds=77777$short", "we have a short salt string but not a short password",
			"$5$rounds=77777$short$JiO1O3ZpDAxGJeaDIuqCoEFysAe1mZNJRs3pw0KQRd/"},
		{"$5$rounds=123456$asaltof16chars..", "a short string",
			"$5$rounds=123456$asaltof16chars..$gP3VQ/6X7UUEW3HkBn2w1/Ptq2jxPyzV/cZKmF/wJvD"},
	}
	for i, c := range cases {
		got := cryptSHA256(c.password, c.salt)
		if got != c.want {
			t.Errorf("case %d: cryptSHA256(%q, %q)\n  got:  %q\n  want: %q",
				i+1, c.password, c.salt, got, c.want)
		}
	}
}

func TestCryptSHA512(t *testing.T) {
	cases := []struct{ salt, password, want string }{
		{"$6$saltstring", "Hello world!",
			"$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1"},
		{"$6$rounds=10000$saltstringsaltstring", "Hello world!",
			"$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v."},
		{"$6$rounds=5000$toolongsaltstring", "This is just a test",
			"$6$rounds=5000$toolongsaltstrin$lQ8jolhgVRVhY4b5pZKaysCLi0QBxGoNeKQzQ3glMhwllF7oGDZxUhx1yxdYcz/e1JSbq3y6JMxxl8audkUEm0"},
		{"$6$rounds=1400$anotherlongsaltstring",
			"a very much longer text to encrypt.  This one even stretches over morethan one line.",
			"$6$rounds=1400$anotherlongsalts$POfYwTEok97VWcjxIiSOjiykti.o/pQs.wPvMxQ6Fm7I6IoYN3CmLs66x9t0oSwbtEW7o7UmJEiDwGqd8p4ur1"},
		{"$6$rounds=77777$short", "we have a short salt string but not a short password",
			"$6$rounds=77777$short$WuQyW2YR.hBNpjjRhpYD/ifIw05xdfeEyQoMxIXbkvr0gge1a1x3yRULJ5CCaUeOxFmtlcGZelFl5CxtgfiAc0"},
		{"$6$rounds=123456$asaltof16chars..", "a short string",
			"$6$rounds=123456$asaltof16chars..$BtCwjqMJGx5hrJhZywWvt0RLE8uZ4oPwcelCjmw2kSYu.Ec6ycULevoBK25fs2xXgMNrCzIMVcgEJAstJeonj1"},
	}
	for i, c := range cases {
		got := cryptSHA512(c.password, c.salt)
		if got != c.want {
			t.Errorf("case %d: cryptSHA512(%q, %q)\n  got:  %q\n  want: %q",
				i+1, c.password, c.salt, got, c.want)
		}
	}
}

func TestCryptMD5(t *testing.T) {
	// From PHP's crypt.phpt test case.
	got := cryptMD5("rasmuslerdorf", "$1$rasmusle$")
	want := "$1$rasmusle$rISCgZzpwk3UhDidwXvin0"
	if got != want {
		t.Errorf("cryptMD5:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestCryptBcryptRoundTrip(t *testing.T) {
	// Generate a valid bcrypt hash, then ensure we reproduce it when given
	// the full hash as salt (the classic verification pattern).
	salt := "$2y$04$abcdefghijklmnopqrstuu"
	h := cryptBcrypt("hello", salt)
	if len(h) != 60 {
		t.Fatalf("expected 60-char bcrypt output, got %d: %q", len(h), h)
	}
	if h[:29] != salt {
		t.Errorf("salt prefix not preserved: got %q, want %q", h[:29], salt)
	}
	// Feeding the full hash back in must produce the same output: this is
	// how PHP's crypt($pw, $hash) === $hash verification works.
	h2 := cryptBcrypt("hello", h)
	if h2 != h {
		t.Errorf("round-trip mismatch:\n  first:  %q\n  second: %q", h, h2)
	}
}

func TestCryptDispatcher(t *testing.T) {
	cases := []struct{ password, salt, want string }{
		{"x", "", "*0"},
		{"x", "*0", "*1"},
		{"x", "*1", "*0"},
		{"test", "$23$04$1234567890123456789012345", "*0"},
		{"test", "$20$04$1234567890123456789012345", "*0"},
		{"test", "$2g$04$1234567890123456789012345", "*0"},
		{"test", "$2a$4$1234567891234567891234567", "*0"},  // single-digit cost
		{"test", "$2a$00$1234567891234567891234567", "*0"}, // below MinCost
		{"test", "$2a$32$1234567891234567891234567", "*0"}, // above MaxCost
		{"foo", "$:#", "*0"},                               // invalid prefix
		{"foo", "$:5zd$01\n", "*0"},
	}
	for _, c := range cases {
		got := cryptDispatch(c.password, c.salt)
		if got != c.want {
			t.Errorf("cryptDispatch(%q, %q) = %q, want %q", c.password, c.salt, got, c.want)
		}
	}
}
