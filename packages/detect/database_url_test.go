package detect

import "testing"

func TestDatabaseURLCredentialsRequired(t *testing.T) {
	for _, input := range []string{
		"postgres://invalid.invalid/db", "mysql://invalid.invalid:3306/db", "postgres://demo@invalid.invalid/db",
		"postgres://invalid.invalid/db?sslmode=require", "postgres://invalid.invalid/db?password=",
	} {
		for _, match := range Detect(input) {
			if match.Kind == Secret {
				t.Fatalf("non-secret URL classified as credential: %q %#v", input, match)
			}
		}
	}
}

func TestCredentialURLFullBoundaries(t *testing.T) {
	for _, value := range []string{
		"postgres://demo:placeholder@invalid.invalid/db",
		"postgresql://demo:placeholder@invalid.invalid/db",
		"POSTGRESQL://demo:placeholder@invalid.invalid/db",
		"mysql://demo:placeholder@invalid.invalid:3306/db",
		"postgresql://demo:p%40ss%3Aword@invalid.invalid/db",
		"postgresql://demo:p%ZZ@invalid.invalid/db", // malformed encoding still carries a credential
		"postgresql://demo:placeholder@[2001:db8::1]:5432/db",
		"postgresql://demo:placeholder@host.invalid:5432,other.invalid:5432/db",
		"postgresql:///db?password=synthetic_value",
		"postgresql:///db?sslpassword=synthetic_value&sslmode=require",
		"mysql://invalid.invalid/db?pass%77ord=synthetic_value",
		"postgres://invalid.invalid/db?access_token=synthetic_value==",
		"postgres://invalid.invalid/db?password=synthetic!",
		"postgres://invalid.invalid/db?password=synthetic)",
		"postgres://invalid.invalid/db?password=synthetic.",
	} {
		input := "🙂 " + value + " end"
		found := false
		for _, m := range Detect(input) {
			if m.Kind == Secret {
				if found || input[m.Start:m.End] != value {
					t.Fatalf("incomplete/extra URL span: %q %#v", value, m)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("missed credential URI: %q", value)
		}
	}
	for _, wrapped := range []struct{ input, value string }{
		{"postgres://demo:placeholder@invalid.invalid/db.", "postgres://demo:placeholder@invalid.invalid/db"},
		{"(postgresql://demo:placeholder@invalid.invalid/db)", "postgresql://demo:placeholder@invalid.invalid/db"},
		{"(postgresql:///db?password=synthetic))", "postgresql:///db?password=synthetic)"},
		{"'postgresql:///db?password=synthetic!'", "postgresql:///db?password=synthetic!"},
		{"\"postgresql:///db?password=synthetic)\"", "postgresql:///db?password=synthetic)"},
	} {
		got := Detect(wrapped.input)
		if len(got) != 1 || got[0].Kind != Secret || wrapped.input[got[0].Start:got[0].End] != wrapped.value {
			t.Fatalf("wrong quoted/prose boundary: %q %#v", wrapped.input, got)
		}
	}
}

func TestDatabaseURIQueryCredentialBoundaries(t *testing.T) {
	for _, input := range []string{
		"postgresql://invalid.invalid/db?sslkey=public-path",
		"postgresql://invalid.invalid/db?notpassword=synthetic",
		"postgresql://invalid.invalid/db#fragment?password=synthetic",
		"postgresql://demo:@invalid.invalid/db", "postgresql:///db?password=&sslmode=require",
	} {
		for _, m := range Detect(input) {
			if m.Kind == Secret {
				t.Fatalf("noncredential URI flagged: %q", input)
			}
		}
	}
}

func TestDatabaseAuthorityIsNotAnEmail(t *testing.T) {
	for _, input := range []string{"postgres://demo@invalid.invalid/db", "mysql://demo@invalid.invalid:3306/db", "POSTGRESQL://demo@invalid.invalid"} {
		if got := Detect(input); len(got) != 0 {
			t.Fatalf("username@host treated as email: %q %#v", input, got)
		}
	}
	for _, input := range []string{"postgresql:///db?user=demo@example.invalid", "postgresql://demo@example.invalid@database.invalid/db", "xpostgres://demo@example.invalid"} {
		found := false
		for _, m := range Detect(input) {
			if m.Kind == Email {
				found = true
			}
		}
		if !found {
			t.Fatalf("email over-suppressed: %q", input)
		}
	}
}
