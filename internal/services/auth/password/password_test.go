package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() err=%v", err)
	}
	if h == "" {
		t.Fatalf("Hash() returned empty hash")
	}

	if err := Verify("correct horse battery staple", h); err != nil {
		t.Fatalf("Verify() expected success, got %v", err)
	}
	if err := Verify("wrong password", h); err == nil {
		t.Fatalf("Verify() expected mismatch")
	} else if err != ErrMismatch {
		t.Fatalf("Verify() expected ErrMismatch, got %v", err)
	}
}

func TestVerifyRejectsInvalidHash(t *testing.T) {
	if err := Verify("pw", "not-a-phc-hash"); err == nil {
		t.Fatalf("expected error")
	}
}
