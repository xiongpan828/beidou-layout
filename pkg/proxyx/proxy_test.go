package proxyx

import "testing"

func TestMethodName(t *testing.T) {
	cases := map[string]string{
		"/service.admin_service.admin_hello.Hello/ListGreeting": "ListGreeting",
		"ListGreeting": "ListGreeting",
		"":             "",
	}
	for op, want := range cases {
		if got := methodName(op); got != want {
			t.Fatalf("methodName(%q) = %q, want %q", op, got, want)
		}
	}
}
