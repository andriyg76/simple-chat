package chatterm

import "testing"

func TestValidateServer(t *testing.T) {
	if err := validateServer("https://chat.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := validateServer("http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if err := validateServer("http://evil.example.com"); err == nil {
		t.Fatal("expected HTTPS requirement for remote host")
	}
}
