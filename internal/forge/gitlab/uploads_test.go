package gitlab

import (
	"net/http"
	"testing"
)

func TestReadUploadUsesTheAPIWithTheToken(t *testing.T) {
	s, c := newServer(t)
	png := []byte("\x89PNG\r\n\x1a\nimage")
	s.on("GET /api/v4/projects/nas%2Fsb/uploads/0123abcd/shot%20one.png", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") == "" && r.Header.Get("Authorization") == "" {
			t.Error("no token sent")
		}
		w.Write(png)
	})
	got, err := c.ReadUpload(ctx, "nas", "sb", "0123abcd", "shot one.png")
	if err != nil || string(got) != string(png) {
		t.Fatalf("ReadUpload = %q, %v", got, err)
	}
}
