package tokens

import "testing"

func TestRefreshToken_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 2000; i++ {
		tok, err := NewRefreshToken()
		if err != nil {
			t.Fatalf("NewRefreshToken: %v", err)
		}
		if tok == "" {
			t.Fatal("empty token")
		}
		if len(tok) < 40 {
			t.Fatalf("token too short: %d", len(tok))
		}
		if _, ok := seen[tok]; ok {
			t.Fatal("duplicate token")
		}
		seen[tok] = struct{}{}
	}
}

func TestHashRefreshToken_Stable(t *testing.T) {
	a := HashRefreshToken("abc")
	b := HashRefreshToken("abc")
	c := HashRefreshToken("abcd")
	if string(a) != string(b) {
		t.Fatal("hash not stable")
	}
	if string(a) == string(c) {
		t.Fatal("hash should differ")
	}
}
