package ping

import "testing"

func TestDefaultPort(t *testing.T) {
	if DefaultPort(EditionBedrock) != 19132 {
		t.Fatalf("expected bedrock default port 19132")
	}
	if DefaultPort(EditionJava) != 25565 {
		t.Fatalf("expected java default port 25565")
	}
}

func TestParsePort(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty", input: "", want: 0, wantErr: false},
		{name: "valid", input: "25565", want: 25565, wantErr: false},
		{name: "trim", input: " 19132 ", want: 19132, wantErr: false},
		{name: "non-numeric", input: "abc", want: 0, wantErr: true},
		{name: "too-low", input: "0", want: 0, wantErr: true},
		{name: "too-high", input: "70000", want: 0, wantErr: true},
	}

	for _, tc := range cases {
		result, err := ParsePort(tc.input)
		if tc.wantErr && err == nil {
			t.Fatalf("expected error for %s", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("unexpected error for %s: %v", tc.name, err)
		}
		if result != tc.want {
			t.Fatalf("unexpected result for %s: %d", tc.name, result)
		}
	}
}
