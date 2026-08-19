package strutil

import "testing"

func TestIsEmpty(t *testing.T) {
	if !IsEmpty("") || !IsEmpty("   ") || IsEmpty("x") {
		t.Fatal("empty")
	}
}

func TestTruncate(t *testing.T) {
	if Truncate("hello", 3) != "hel" {
		t.Fatal("trunc")
	}
	if Truncate("hi", 10) != "hi" {
		t.Fatal("no")
	}
}

func TestCamelCase(t *testing.T) {
	if CamelCase("hello world") != "helloWorld" {
		t.Fatal("camel")
	}
	if CamelCase("foo-bar_baz") != "fooBarBaz" {
		t.Fatal("camel2")
	}
}

func TestSnakeCase(t *testing.T) {
	if SnakeCase("Hello World") != "hello_world" {
		t.Fatal("snake")
	}
	if SnakeCase("foo-bar") != "foo_bar" {
		t.Fatal("snake2")
	}
}

func TestContainsAny(t *testing.T) {
	if !ContainsAny("hello world", "hi", "world") {
		t.Fatal("any")
	}
	if ContainsAny("abc", "x", "y") {
		t.Fatal("any2")
	}
}

func TestReverse(t *testing.T) {
	if Reverse("hello") != "olleh" {
		t.Fatal("rev")
	}
}

func TestWordCount(t *testing.T) {
	if WordCount("hello world foo") != 3 {
		t.Fatal("count")
	}
}

func TestMaskEmail(t *testing.T) {
	if MaskEmail("alice@example.com") != "a****@example.com" {
		t.Fatal("mask")
	}
}
