package dto

import (
	"encoding/json"
	"testing"
)

// TestResponseSuccessAlwaysCarriesData 校验空成功响应也保留 data 字段，
// 保证 `{code, data, message}` 契约对前端稳定。
func TestResponseSuccessAlwaysCarriesData(t *testing.T) {
	raw, err := json.Marshal(ResponseSuccess())
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := payload["data"]
	if !ok {
		t.Fatalf("success envelope missing data field: %s", raw)
	}
	if string(data) != "null" {
		t.Fatalf("empty success data = %s, want null", data)
	}
}
