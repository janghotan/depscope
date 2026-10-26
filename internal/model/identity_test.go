package model_test

import (
	"strings"
	"testing"

	"github.com/janghotan/depscope/internal/model"
)

func TestNewPackageIdentityCanonicalizesNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ecosystem model.Ecosystem
		input     string
		want      string
	}{
		{name: "npm lowercase", ecosystem: model.EcosystemNPM, input: "Lodash", want: "lodash"},
		{name: "npm scoped", ecosystem: model.EcosystemNPM, input: "@Scope/Package", want: "@scope/package"},
		{name: "npm scoped with dots", ecosystem: model.EcosystemNPM, input: "@scope/pkg.js", want: "@scope/pkg.js"},
		{name: "pypi separators", ecosystem: model.EcosystemPyPI, input: "Foo_Bar.baz", want: "foo-bar-baz"},
		{name: "pypi repeated separators", ecosystem: model.EcosystemPyPI, input: "Foo..__Bar", want: "foo-bar"},
		{name: "pypi leading digit", ecosystem: model.EcosystemPyPI, input: "3to2", want: "3to2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := model.NewPackageIdentity(tc.ecosystem, tc.input)
			if err != nil {
				t.Fatalf("NewPackageIdentity() error = %v", err)
			}
			if got.Name != tc.want {
				t.Fatalf("Name = %q, want %q", got.Name, tc.want)
			}
			if got.Ecosystem != tc.ecosystem {
				t.Fatalf("Ecosystem = %q, want %q", got.Ecosystem, tc.ecosystem)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestNewPackageIdentityRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ecosystem model.Ecosystem
		input     string
	}{
		{name: "unsupported ecosystem", ecosystem: model.Ecosystem("maven"), input: "guava"},
		{name: "empty", ecosystem: model.EcosystemNPM, input: ""},
		{name: "whitespace", ecosystem: model.EcosystemNPM, input: " lodash"},
		{name: "unscoped slash", ecosystem: model.EcosystemNPM, input: "scope/pkg"},
		{name: "scope without package", ecosystem: model.EcosystemNPM, input: "@scope/"},
		{name: "scope without slash", ecosystem: model.EcosystemNPM, input: "@scope"},
		{name: "pypi leading hyphen", ecosystem: model.EcosystemPyPI, input: "-bad"},
		{name: "pypi internal space", ecosystem: model.EcosystemPyPI, input: "foo bar"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := model.NewPackageIdentity(tc.ecosystem, tc.input); err == nil {
				t.Fatal("NewPackageIdentity() error = nil, want error")
			}
		})
	}
}

func TestPackageIdentityValidateRejectsNonCanonicalJSONValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value model.PackageIdentity
	}{
		{
			name:  "uppercase npm name",
			value: model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "Lodash"},
		},
		{
			name:  "unnormalized pypi name",
			value: model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "Foo_Bar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.value.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), "canonical") {
				t.Fatalf("Validate() error = %q, want canonical-name error", err)
			}
		})
	}
}

func TestPackageIdentityString(t *testing.T) {
	t.Parallel()

	identity := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "@scope/pkg"}
	if got, want := identity.String(), "npm:@scope/pkg"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
