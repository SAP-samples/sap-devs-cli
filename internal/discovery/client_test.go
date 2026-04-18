package discovery

import (
	"testing"
)

func TestExtractBatchJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "multipart with trailing boundary",
			body: "--batch_123\r\nContent-Type: application/http\r\n\r\nHTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n" +
				`{"d":{"GetStuff":"[{\"id\":1}]"}}` +
				"\r\n--batch_123--\r\n",
			want: `[{"id":1}]`,
		},
		{
			name: "double-encoded JSON string value",
			body: `--batch_abc` + "\r\n\r\n" +
				`{"d":{"Fn":"[{\"a\":\"b\"}]"}}` +
				"\r\n--batch_abc--\r\n",
			want: `[{"a":"b"}]`,
		},
		{
			name: "raw JSON value (not double-encoded)",
			body: `--batch_abc` + "\r\n\r\n" +
				`{"d":{"Fn":{"key":"val"}}}` +
				"\r\n--batch_abc--\r\n",
			want: `{"key":"val"}`,
		},
		{
			name:    "no JSON at all",
			body:    "--batch_abc\r\nno json here\r\n--batch_abc--\r\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBatchJSON([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", string(got), tt.want)
			}
		})
	}
}
