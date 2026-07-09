package identity

import "testing"

func TestNewUUIDReturnsValidVersion4UUID(t *testing.T) {
	id, err := NewUUID()
	if err != nil {
		t.Fatalf("NewUUID returned error: %v", err)
	}

	if err := ValidateUUID(id); err != nil {
		t.Fatalf("generated UUID is invalid: %v", err)
	}
	if len(id) != UUIDLength {
		t.Fatalf("expected UUID length %d, got %d", UUIDLength, len(id))
	}
	if id[14] != '4' {
		t.Fatalf("expected version 4 UUID, got %q", id[14])
	}
	if id[19] != '8' && id[19] != '9' && id[19] != 'a' && id[19] != 'b' {
		t.Fatalf("expected RFC 4122 variant, got %q", id[19])
	}
}

func TestNewUUIDGeneratesDistinctValues(t *testing.T) {
	first, err := NewUUID()
	if err != nil {
		t.Fatalf("first NewUUID returned error: %v", err)
	}
	second, err := NewUUID()
	if err != nil {
		t.Fatalf("second NewUUID returned error: %v", err)
	}

	if first == second {
		t.Fatalf("expected generated UUIDs to differ, got %q", first)
	}
}

func TestNormalizeUUIDTrimsAndLowercases(t *testing.T) {
	input := "  123E4567-E89B-42D3-A456-426614174000  "
	got, err := NormalizeUUID(input)
	if err != nil {
		t.Fatalf("NormalizeUUID returned error: %v", err)
	}

	want := "123e4567-e89b-42d3-a456-426614174000"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestValidateUUIDRejectsInvalidValues(t *testing.T) {
	cases := []string{
		"",
		"not-a-uuid",
		"123e4567-e89b-12d3-a456-426614174000", // wrong version
		"123e4567-e89b-42d3-c456-426614174000", // wrong variant
		"123e4567_e89b_42d3_a456_426614174000", // wrong separators
		"123e4567-e89b-42d3-a456-42661417400z", // non-hex
	}

	for _, input := range cases {
		if err := ValidateUUID(input); err == nil {
			t.Fatalf("expected %q to be invalid", input)
		}
	}
}

func TestIsUUID(t *testing.T) {
	if !IsUUID("123e4567-e89b-42d3-a456-426614174000") {
		t.Fatal("expected valid UUID")
	}
	if IsUUID("123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("expected wrong-version UUID to be invalid")
	}
}
