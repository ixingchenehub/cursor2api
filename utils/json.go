package utils

import (
	"bytes"

	jsoniter "github.com/json-iterator/go"
)

// MarshalToString JSON编码为字符串
func MarshalToString(v any) string {
	s, err := jsoniter.MarshalToString(v)
	if err != nil {
		return ""
	}
	return s
}

// MarshalToBytes JSON编码为字节数组
func MarshalToBytes(v any) []byte {
	s, err := jsoniter.Marshal(v)
	if err != nil {
		return []byte{}
	}
	return s
}

// MarshalIndentToString JSON编码为格式化字符串
func MarshalIndentToString(v any) string {
	bf := bytes.NewBuffer([]byte{})
	encoder := jsoniter.NewEncoder(bf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "\t")
	_ = encoder.Encode(v)
	return bf.String()
}
