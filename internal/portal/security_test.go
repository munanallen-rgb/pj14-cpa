package portal

import "testing"

func TestDeriveSub2APIPasswordStableAndScoped(t *testing.T) {
	first := deriveSub2APIPassword("secret-a", 10)
	second := deriveSub2APIPassword("secret-a", 10)
	otherUser := deriveSub2APIPassword("secret-a", 11)
	otherSecret := deriveSub2APIPassword("secret-b", 10)

	if first == "" {
		t.Fatal("derived password is empty")
	}
	if first != second {
		t.Fatal("derived password is not stable for the same secret and user")
	}
	if first == otherUser {
		t.Fatal("derived password should differ for another user")
	}
	if first == otherSecret {
		t.Fatal("derived password should differ for another secret")
	}
}

func TestKeyPreview(t *testing.T) {
	got := keyPreview("sk-abcdefghijklmnopqrstuvwxyz")
	if got != "sk-abc...wxyz" {
		t.Fatalf("keyPreview = %q, want %q", got, "sk-abc...wxyz")
	}
}
