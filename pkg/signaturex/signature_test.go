package signaturex

import "testing"

func TestSignValid(t *testing.T) {
	sig := Sign("secret-0123456789", "PUT", "a/b.jpg", 100)
	if !Valid("secret-0123456789", "PUT", "a/b.jpg", 100, sig) {
		t.Fatal("valid signature rejected")
	}
	for name, ok := range map[string]bool{
		"method": Valid("secret-0123456789", "GET", "a/b.jpg", 100, sig),
		"key":    Valid("secret-0123456789", "PUT", "a/c.jpg", 100, sig),
		"expire": Valid("secret-0123456789", "PUT", "a/b.jpg", 101, sig),
		"secret": Valid("other-secret-0123", "PUT", "a/b.jpg", 100, sig),
		"hex":    Valid("secret-0123456789", "PUT", "a/b.jpg", 100, "zz"),
	} {
		if ok {
			t.Errorf("tampered %s accepted", name)
		}
	}
}
