package core

import (
	"encoding/json"
	"net/http"
	"net/textproto"
)

// 客戶端發送的元信息
type ClientMetadata struct {
	URL    string      `json:"url"`
	Method string      `json:"method"`
	Header http.Header `json:"header"`
}

func (m *ClientMetadata) Unmarshal(data []byte) (e error) {
	e = json.Unmarshal(data, m)
	if e != nil {
		return
	}
	if m.Header != nil {
		keys := make([]string, 0, len(m.Header))
		for k := range m.Header {
			keys = append(keys, k)
		}
		for _, k0 := range keys {
			k1 := textproto.CanonicalMIMEHeaderKey(k0)
			if k1 != k0 {
				m.Header[k1] = m.Header[k0]
				delete(m.Header, k0)
			}
		}
	}
	return
}

// 服務器發送的元信息
type ServerMetadata struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
}
