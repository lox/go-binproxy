package bintest_test

import (
	"os/exec"
	"testing"

	"github.com/lox/bintest"
)

func TestCallingMockWithNoExpectationsSet(t *testing.T) {
	m, err := bintest.NewMock("test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = exec.Command(m.Path, "blargh").CombinedOutput()
	if err == nil {
		t.Errorf("Expected a failure without any expectations set")
	}

	if m.Check(t) == false {
		t.Errorf("Assertions should have passed (there were none)")
	}
}

func TestCallingMockWithExpectationsSet(t *testing.T) {
	m, err := bintest.NewMock("test")
	if err != nil {
		t.Fatal(err)
	}

	m.Expect("blargh").
		AndWriteToStdout("llamas rock").
		AndExitWith(0)

	out, err := exec.Command(m.Path, "blargh").CombinedOutput()
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	if string(out) != "llamas rock" {
		t.Fatalf("Unexpected output %q", out)
	}

	if m.Check(t) == false {
		t.Errorf("Assertions should have passed")
	}
}

func TestMockWithPassthroughToLocalCommand(t *testing.T) {
	m, err := bintest.NewMock("echo")
	if err != nil {
		t.Fatal(err)
	}

	m.PassthroughToLocalCommand()
	m.Expect("hello", "world")

	out, err := exec.Command(m.Path, "hello", "world").CombinedOutput()
	if err != nil {
		t.Logf("Output: %s", out)
		t.Fatal(err)
	}

	if string(out) != "hello world\n" {
		t.Fatalf("Unexpected output %q", out)
	}

	m.Check(t)
}

func TestArgumentsThatDontMatch(t *testing.T) {
	var testCases = []struct {
		expected bintest.Arguments
		actual   []string
	}{
		{
			bintest.Arguments{"test", "llamas", "rock"},
			[]string{"test", "llamas", "alpacas"},
		},
		{
			bintest.Arguments{"test", "llamas"},
			[]string{"test", "llamas", "alpacas"},
		},
	}

	for _, test := range testCases {
		match, _ := test.expected.Match(test.actual...)
		if match {
			t.Fatalf("Expected %v and %v to not match", test.expected, test.actual)
		}
	}
}

func TestArgumentsThatMatch(t *testing.T) {
	var testCases = []struct {
		expected bintest.Arguments
		actual   []string
	}{
		{
			bintest.Arguments{"test", "llamas", "rock"},
			[]string{"test", "llamas", "rock"},
		},
		{
			bintest.Arguments{"test", "llamas", bintest.MatchAny()},
			[]string{"test", "llamas", "rock"},
		},
	}

	for _, test := range testCases {
		match, _ := test.expected.Match(test.actual...)
		if !match {
			t.Fatalf("Expected %v and %v to match", test.expected, test.actual)
		}
	}
}

func TestArgumentsToString(t *testing.T) {
	var testCases = []struct {
		args     bintest.Arguments
		expected string
	}{
		{
			bintest.Arguments{"test", "llamas", "rock"},
			`"test" "llamas" "rock"`,
		},
		{
			bintest.Arguments{"test", "llamas", bintest.MatchAny()},
			`"test" "llamas" *`,
		},
	}

	for _, test := range testCases {
		actual := test.args.String()
		if actual != test.expected {
			t.Fatalf("Expected [%s], got [%s]", test.expected, actual)
		}
	}
}
