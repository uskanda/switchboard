package fake

import (
	"context"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// A GitLab fake names its issues gl:, keeps subgroups in the owner, counts internal as not private, can lose the
// editor of a body, and, like gl.uskan.dev (V22), does not touch an issue's update time when a note is added.
func TestGitLabFake(t *testing.T) {
	clock := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	f := NewGitLab(func() time.Time { return clock })
	me := f.AddUser("uskanda", 1)
	f.AddProject("nas/tools", "sandbox", "private")
	f.AddProject("nas", "shared", "internal")
	ref := f.AddIssue("nas/tools", "sandbox", 3, "t", "b", me)
	if ref.Kind() != forge.GitLab || ref.String() != "gl:nas/tools/sandbox#3" {
		t.Fatalf("ref = %s", ref)
	}
	ctx := context.Background()
	if p, _ := f.RepoPrivate(ctx, "nas/tools", "sandbox"); !p {
		t.Error("a private project is not private")
	}
	if p, _ := f.RepoPrivate(ctx, "nas", "shared"); p {
		t.Error("an internal project counts as private")
	}

	clock = clock.Add(time.Minute)
	f.AddComment(ref, "a reply", me)
	is, _, _ := f.Thread(ctx, ref)
	if !is.UpdatedAt.Equal(clock.Add(-time.Minute)) {
		t.Errorf("a note moved the update time to %s", is.UpdatedAt)
	}

	f.LoseBodyEditor(ref, "edited")
	is, _, _ = f.Thread(ctx, ref)
	if !is.BodyEdit.Edited || !is.BodyEdit.Unknown || is.BodyEdit.Editor != nil || is.Body != "edited" {
		t.Errorf("BodyEdit = %+v, body %q", is.BodyEdit, is.Body)
	}
}
