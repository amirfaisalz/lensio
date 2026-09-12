package providers

// SetJSONMarshalForTest overrides jsonMarshal for unit testing error branches.
func SetJSONMarshalForTest(fn func(v any) ([]byte, error)) func() {
	orig := jsonMarshal
	jsonMarshal = fn
	return func() { jsonMarshal = orig }
}
