package memory

import (
	"strings"
	"testing"
)

func TestSignatureStableAcrossMachines(t *testing.T) {
	a := `Error: Unable to resolve module ./App from /Users/alice/dev/shop/index.js: ./App could not be found
    at ModuleResolver.resolveDependency (/Users/alice/dev/shop/node_modules/metro/src/node-haste/DependencyGraph/ModuleResolution.js:144:15)`
	b := `Error: Unable to resolve module ./App from /home/bob/work/app/index.js: ./App could not be found
    at ModuleResolver.resolveDependency (/home/bob/work/app/node_modules/metro/src/ModuleResolution.js:151:9)`
	if Signature(a, "") != Signature(b, "") {
		t.Fatalf("same error, different signatures:\n%s\n%s", Normalize(a), Normalize(b))
	}
	c := `Error: Unable to resolve module ./Home from /home/bob/work/app/index.js: ./Home could not be found`
	if Signature(a, "") == Signature(c, "") {
		t.Fatal("different modules share a signature")
	}
	v1 := "npm ERR! peer dep missing: react@18.2.0, required by react-native@0.76.1"
	v2 := "npm ERR! peer dep missing: react@18.3.1, required by react-native@0.76.5"
	if Signature(v1, "") != Signature(v2, "") {
		t.Fatalf("versions not normalized: %q vs %q", Normalize(v1), Normalize(v2))
	}
	if len(Signature("", "how to rotate logs")) != 10 {
		t.Fatal("signature length")
	}
}

func TestErrorLinePicksTheError(t *testing.T) {
	log := "Starting build\nCompiling 42 files\nerror[E0382]: borrow of moved value: `x`\n --> src/main.rs:4:20"
	if got := errorLine(log); !strings.HasPrefix(got, "error[E0382]") {
		t.Fatalf("errorLine = %q", got)
	}
}

func TestLooksLikeError(t *testing.T) {
	for q, want := range map[string]bool{
		"TypeError: Cannot read properties of undefined (reading 'map')": true,
		"Error: listen EADDRINUSE: address already in use :::3000":       true,
		"how do I configure the ingress controller":                      false,
		"rollback procedure": false,
	} {
		if got := LooksLikeError(q); got != want {
			t.Errorf("LooksLikeError(%q) = %v", q, got)
		}
	}
}

func TestMaskSecrets(t *testing.T) {
	in := strings.Join([]string{
		"export OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwx",
		"curl -H 'Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456'",
		"git clone https://ghp_abcdefghijklmnopqrstuvwxyz0123456789@github.com/x/y",
		"DATABASE_URL=postgres://admin:hunter22secret@db:5432/app",
		"password: correcthorse",
		"aws AKIAABCDEFGHIJKLMNOP",
	}, "\n")
	out := MaskSecrets(in)
	for _, leak := range []string{"abcdefghijklmnopqrstuvwx", "abcdefghijklmnopqrstuvwxyz123456", "ghp_", "hunter22secret", "correcthorse", "AKIAABCDEFGHIJKLMNOP"} {
		if strings.Contains(out, leak) {
			t.Fatalf("secret %q left in:\n%s", leak, out)
		}
	}
	for _, kept := range []string{"OPENAI_API_KEY=", "Bearer [secret]", "postgres://admin:[secret]@db:5432/app", "password: [secret]"} {
		if !strings.Contains(out, kept) {
			t.Fatalf("%q missing from:\n%s", kept, out)
		}
	}
}
