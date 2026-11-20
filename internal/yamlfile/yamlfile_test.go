package yamlfile

import "testing"

func TestJavaDouble(t *testing.T) {
	for in, want := range map[float64]string{
		0: "0.0", 1: "1.0", -2.5: "-2.5", 0.1: "0.1", 1e7: "1.0E7", 1.25e-4: "1.25E-4", 123456.5: "123456.5",
	} {
		if got := JavaDouble(in); got != want {
			t.Errorf("JavaDouble(%v) = %s, want %s", in, got, want)
		}
	}
}

func TestDefaultsAreWrittenBack(t *testing.T) {
	f := New("", true)
	if err := f.Parse([]byte("a:\n    b: 3\n")); err != nil {
		t.Fatal(err)
	}
	if f.Int("a.b", 1) != 3 || f.Bool("a.c", true) != true || f.Double("x.y", 2) != 2 {
		t.Fatal("wrong values read")
	}
	f.StringList("list", []string{"on", "1"})
	f.Header = "# head"
	data, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "# head\na:\n    b: 3\n    c: true\nx:\n    y: 2.0\nlist:\n    - 'on'\n    - '1'\n"
	if got := string(data); got != want {
		t.Fatalf("encoded:\n%s\nwant:\n%s", got, want)
	}
}
