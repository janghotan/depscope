package inventory

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeRunner 记录命令调用并按固定键返回输出或错误。
type fakeRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if output, ok := r.outputs[key]; ok {
		return output, nil
	}
	return nil, errors.New("unexpected command: " + key)
}

func TestSystemNPMGlobalRoot(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{outputs: map[string][]byte{
		"npm root -g": []byte("  /usr/lib/node_modules\n"),
	}}
	system := System{Timeout: time.Second, Runner: runner}
	root, err := system.NPMGlobalRoot(context.Background())
	if err != nil {
		t.Fatalf("NPMGlobalRoot: %v", err)
	}
	if root != "/usr/lib/node_modules" {
		t.Fatalf("root = %q, want /usr/lib/node_modules", root)
	}
	if want := []string{"npm root -g"}; !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %v, want %v", runner.calls, want)
	}
}

func TestSystemNPMGlobalRootRejectsUnusableOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output []byte
		err    error
	}{
		{name: "empty", output: []byte(" \n")},
		{name: "multiple lines", output: []byte("/first\n/second\n")},
		{name: "relative", output: []byte("node_modules\n")},
		{name: "command failed", err: errors.New("exec: npm: not found")},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{
				outputs: map[string][]byte{"npm root -g": test.output},
				errs:    map[string]error{},
			}
			if test.err != nil {
				runner.errs["npm root -g"] = test.err
			}
			system := System{Timeout: time.Second, Runner: runner}
			if _, err := system.NPMGlobalRoot(context.Background()); err == nil {
				t.Fatal("NPMGlobalRoot error = nil, want failure")
			}
		})
	}
}

func TestSystemPipPackagesUsesFixture(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(fixturePath("inventory", "python", "pip-list.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	key := "python3 -m pip list --format=json --disable-pip-version-check"
	runner := &fakeRunner{outputs: map[string][]byte{key: data}}
	system := System{Timeout: time.Second, Runner: runner}
	packages, err := system.PipPackages(context.Background())
	if err != nil {
		t.Fatalf("PipPackages: %v", err)
	}
	want := []Package{
		{Name: "Safe_Pkg", Version: "1.0.0"},
		{Name: "vulnerable-pkg", Version: "2.3.4"},
		{Name: "pip", Version: "24.0"},
	}
	if !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %v, want %v", packages, want)
	}
	if wantCalls := []string{key}; !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", runner.calls, wantCalls)
	}
}

func TestSystemPipPackagesFallsBackToPython(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(fixturePath("inventory", "python", "pip-list.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	python3Key := "python3 -m pip list --format=json --disable-pip-version-check"
	pythonKey := "python -m pip list --format=json --disable-pip-version-check"
	runner := &fakeRunner{
		outputs: map[string][]byte{pythonKey: data},
		errs:    map[string]error{python3Key: errors.New("python3 not found")},
	}
	system := System{Timeout: time.Second, Runner: runner}
	packages, err := system.PipPackages(context.Background())
	if err != nil {
		t.Fatalf("PipPackages: %v", err)
	}
	if len(packages) != 3 {
		t.Fatalf("packages = %d, want 3", len(packages))
	}
	if want := []string{python3Key, pythonKey}; !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %v, want %v", runner.calls, want)
	}
}

func TestSystemPipPackagesReportsBothFailures(t *testing.T) {
	t.Parallel()

	python3Key := "python3 -m pip list --format=json --disable-pip-version-check"
	pythonKey := "python -m pip list --format=json --disable-pip-version-check"
	runner := &fakeRunner{errs: map[string]error{
		python3Key: errors.New("python3 unavailable"),
		pythonKey:  errors.New("python unavailable"),
	}}
	system := System{Timeout: time.Second, Runner: runner}
	_, err := system.PipPackages(context.Background())
	if err == nil {
		t.Fatal("PipPackages error = nil, want combined failure")
	}
	if !strings.Contains(err.Error(), "python3") || !strings.Contains(err.Error(), "python") {
		t.Fatalf("error = %q, want both interpreter names", err)
	}
}

func TestSystemPipPackagesRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	key := "python3 -m pip list --format=json --disable-pip-version-check"
	runner := &fakeRunner{outputs: map[string][]byte{key: []byte("not json")}}
	system := System{Timeout: time.Second, Runner: runner}
	_, err := system.PipPackages(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pip list") {
		t.Fatalf("PipPackages error = %v, want pip list parse failure", err)
	}
}

func TestParsePipListRejectsMalformedDocuments(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "null", "[] {}", `[{"name":"x","version":"1.0.0"}] extra`} {
		if _, err := parsePipList([]byte(input)); err == nil {
			t.Fatalf("parsePipList(%q) error = nil, want failure", input)
		}
	}
	packages, err := parsePipList([]byte("[]"))
	if err != nil {
		t.Fatalf("parsePipList empty array: %v", err)
	}
	if len(packages) != 0 {
		t.Fatalf("packages = %v, want empty", packages)
	}
}

func TestOutputLimiterEnforcesLimit(t *testing.T) {
	t.Parallel()

	writer := &outputLimiter{limit: 4}
	if _, err := writer.Write([]byte("abcd")); err != nil {
		t.Fatalf("Write within limit: %v", err)
	}
	if _, err := writer.Write([]byte("e")); err == nil {
		t.Fatal("Write beyond limit error = nil, want failure")
	}
	if !writer.overflow {
		t.Fatal("overflow = false, want true")
	}
	if got := string(writer.data); got != "abcd" {
		t.Fatalf("data = %q, want abcd", got)
	}
}
