// Resource attribute helpers.
package otlp

import "strconv"

type KV struct{ Key, Value string }

func Attr(k, v string) KV { return KV{Key: k, Value: v} }

func Map(kvs ...KV) map[string]string {
	if len(kvs) == 0 { return nil }
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs { out[kv.Key] = kv.Value }
	return out
}

func Merge(a, b map[string]string) map[string]string {
	if a == nil && b == nil { return nil }
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a { out[k] = v }
	for k, v := range b { out[k] = v }
	return out
}

func String(k, v string) KV { return Attr(k, v) }
func Int(k string, i int64) KV { return Attr(k, strconv.FormatInt(i, 10)) }
func Float(k string, f float64) KV {
	return Attr(k, strconv.FormatFloat(f, byte(0x66), -1, 64))
}
func Bool(k string, b bool) KV {
	if b { return Attr(k, "true") }
	return Attr(k, "false")
}
