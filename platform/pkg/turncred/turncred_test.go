package turncred

import "testing"

func TestGenerate(t *testing.T) {
	c, err := Generate("shared-secret", "user-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Username == "" || c.Password == "" {
		t.Fatal("empty credential")
	}
}
