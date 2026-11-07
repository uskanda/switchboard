package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrConfigChanged is returned when the file changed between OpenEditor and the write: someone edited it by hand.
var ErrConfigChanged = errors.New("the configuration file was changed by someone else; reload and try again")

// ErrProjectExists is returned when the project to add is configured already.
var ErrProjectExists = errors.New("the project is configured already")

// NewProject is a project added through the inbox service. These are the only keys written; the rest keep their
// defaults until edited by hand.
type NewProject struct {
	Ref        string
	Path       string
	Host       string
	BaseBranch string
	PlanFirst  PlanFirst // written as never when empty
}

// Editor adds projects to a configuration file. It writes the new item as text after the last project, so that
// comments, blank lines and the order of the keys stay as written; yaml.v3 would reflow them on a round trip.
type Editor struct {
	path string // as given, perhaps a symlink
	real string // the file written
	data []byte // the content read by OpenEditor
	root yaml.Node
	cfg  *Config
}

// OpenEditor reads and validates the configuration at path. A later AddProject fails with ErrConfigChanged when
// the file no longer holds what was read here.
func OpenEditor(path string) (*Editor, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	cfg, err := parse(path, data)
	if err != nil {
		return nil, err
	}
	e := &Editor{path: path, real: real, data: data, cfg: cfg}
	if err := yaml.Unmarshal(data, &e.root); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return e, nil
}

// Config is the configuration as read by OpenEditor.
func (e *Editor) Config() *Config { return e.cfg }

// ProjectPath is where a project named repo is cloned: under inbox.project_root as written, so that a root given
// as ~/repos gives ~/repos/<repo>. It is "" when inbox.project_root is not set.
func (e *Editor) ProjectPath(repo string) string {
	root, ok := scalarAt(&e.root, "inbox", "project_root")
	if !ok || root == "" {
		return ""
	}
	return filepath.Join(root, repo)
}

// AddProject appends np to projects. The result must validate and leave every other setting as it was; the
// original is kept as <path>.bak, and the file is replaced in one rename.
func (e *Editor) AddProject(np NewProject) error {
	for _, p := range e.cfg.Projects {
		if p.Ref == np.Ref {
			return fmt.Errorf("%s: %w", np.Ref, ErrProjectExists)
		}
	}
	data, err := e.insert(np)
	if err != nil {
		return err
	}
	next, err := parse(e.path, data)
	if err != nil {
		return err
	}
	if err := e.sameExceptAdded(next, np); err != nil {
		return err
	}
	now, err := os.ReadFile(e.real)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	if !bytes.Equal(now, e.data) {
		return ErrConfigChanged
	}
	fi, err := os.Stat(e.real)
	if err != nil {
		return err
	}
	if err := os.WriteFile(e.path+".bak", e.data, fi.Mode().Perm()); err != nil {
		return fmt.Errorf("back up the configuration: %w", err)
	}
	if err := replace(e.real, data, fi.Mode().Perm()); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	e.data, e.cfg = data, next
	return yaml.Unmarshal(data, &e.root)
}

// insert returns the file with np written as the last item of projects, in the indentation of the first item.
func (e *Editor) insert(np NewProject) ([]byte, error) {
	seq := mappingValue(&e.root, "projects")
	if seq == nil || seq.Kind != yaml.SequenceNode || len(seq.Content) == 0 {
		return nil, errors.New("projects is not a list in the configuration")
	}
	if seq.Style&yaml.FlowStyle != 0 {
		return nil, errors.New("projects is written in flow style ([...]); write it as a block list to add projects")
	}
	lines := strings.SplitAfter(string(e.data), "\n")
	first := seq.Content[0]
	dash := strings.Index(lines[first.Line-1], "-")
	key := first.Column - 1
	if dash < 0 || dash >= key {
		return nil, errors.New("cannot find the indentation of the projects list")
	}
	item := []struct{ k, v string }{{"ref", np.Ref}, {"path", np.Path}, {"host", np.Host}, {"base_branch", np.BaseBranch}}
	var b strings.Builder
	for i, kv := range item {
		if i == 0 {
			b.WriteString(strings.Repeat(" ", dash) + "-" + strings.Repeat(" ", key-dash-1))
		} else {
			b.WriteString(strings.Repeat(" ", key))
		}
		b.WriteString(kv.k + ": " + scalar(kv.v) + "\n")
	}
	fmt.Fprintf(&b, "%splan_first: %s\n", strings.Repeat(" ", key), np.PlanFirst.Mode())

	end := lastLine(seq.Content[len(seq.Content)-1])
	if end > len(lines) {
		end = len(lines)
	}
	if !strings.HasSuffix(lines[end-1], "\n") {
		lines[end-1] += "\n"
	}
	out := strings.Join(lines[:end], "") + b.String() + strings.Join(lines[end:], "")
	return []byte(out), nil
}

// sameExceptAdded checks that next is the configuration read by OpenEditor with np appended to projects, and
// nothing else changed.
func (e *Editor) sameExceptAdded(next *Config, np NewProject) error {
	old := *e.cfg
	got := *next
	n := len(old.Projects)
	if len(got.Projects) != n+1 || !reflect.DeepEqual(got.Projects[:n], old.Projects) {
		return errors.New("adding the project would change the projects already configured")
	}
	added := got.Projects[n]
	if added.Ref != np.Ref || added.Host != np.Host || added.BaseBranch != np.BaseBranch || added.PlanFirst != np.PlanFirst.Mode() ||
		added.Path != expandHome(np.Path) {
		return errors.New("the added project does not read back as written")
	}
	old.Projects, got.Projects = nil, nil
	if !reflect.DeepEqual(old, got) {
		return errors.New("adding the project would change other settings")
	}
	return nil
}

// replace writes data to a temporary file next to path and renames it over path.
func replace(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // after a successful rename there is nothing left to remove
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// mappingValue returns the value of key in the document's top mapping, or nil.
func mappingValue(root *yaml.Node, key string) *yaml.Node {
	n := root
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// lastLine is the last line (1-based) n's content reaches. A block scalar (| or >) runs past its own line.
func lastLine(n *yaml.Node) int {
	end := n.Line
	if n.Kind == yaml.ScalarNode && n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		end += strings.Count(strings.TrimSuffix(n.Value, "\n"), "\n") + 1
	}
	for _, c := range n.Content {
		if l := lastLine(c); l > end {
			end = l
		}
	}
	return end
}

// scalar renders v as a YAML scalar, quoted only when it needs to be.
func scalar(v string) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%q", v)
	}
	return strings.TrimSuffix(string(b), "\n")
}
