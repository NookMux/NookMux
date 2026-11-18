package i18n

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"
)

var placeholderPattern = regexp.MustCompile(`\{\{\.[A-Za-z0-9_]+\}\}`)

func loadLocaleYAML(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	m := make(map[string]string)
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", path, err)
	}
	return m
}

func extractMessageKeyConstants(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "keys.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse keys.go: %v", err)
	}

	keys := make(map[string]string)
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, ident := range vs.Names {
				if len(vs.Values) <= i {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				val, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("failed to unquote %s: %v", ident.Name, err)
				}
				if val != "" {
					keys[ident.Name] = val
				}
			}
		}
	}
	if len(keys) == 0 {
		t.Fatal("expected non-empty message key constants in keys.go")
	}
	return keys
}

func extractPlaceholders(s string) []string {
	matches := placeholderPattern.FindAllString(s, -1)
	if len(matches) == 0 {
		return nil
	}
	uniq := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		uniq[m] = struct{}{}
	}
	out := make([]string, 0, len(uniq))
	for k := range uniq {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestLocaleParityAndConstants(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("i18n.Init() failed: %v", err)
	}

	constKeys := extractMessageKeyConstants(t)
	enMap := loadLocaleYAML(t, "locales/en.yaml")
	zhMap := loadLocaleYAML(t, "locales/zh.yaml")

	if len(enMap) == 0 || len(zhMap) == 0 {
		t.Fatalf("translations must not be empty: en=%d zh=%d", len(enMap), len(zhMap))
	}

	seenConstValues := make(map[string]string, len(constKeys))
	for constName, key := range constKeys {
		if prev, dup := seenConstValues[key]; dup {
			t.Errorf("duplicate message key value %q in constants %s and %s", key, prev, constName)
		}
		seenConstValues[key] = constName

		if _, ok := enMap[key]; !ok {
			t.Errorf("constant %s (%q) missing in locales/en.yaml", constName, key)
		}
		if _, ok := zhMap[key]; !ok {
			t.Errorf("constant %s (%q) missing in locales/zh.yaml", constName, key)
		}
	}

	for key, enVal := range enMap {
		zhVal, ok := zhMap[key]
		if !ok {
			t.Errorf("key %q present in en.yaml but missing in zh.yaml", key)
			continue
		}
		if enVal == "" {
			t.Errorf("key %q has empty translation in en.yaml", key)
		}
		if zhVal == "" {
			t.Errorf("key %q has empty translation in zh.yaml", key)
		}
		enPH := extractPlaceholders(enVal)
		zhPH := extractPlaceholders(zhVal)
		if len(enPH) != len(zhPH) {
			t.Errorf("placeholder count mismatch for %q: en=%v zh=%v", key, enPH, zhPH)
			continue
		}
		for i := range enPH {
			if enPH[i] != zhPH[i] {
				t.Errorf("placeholder mismatch for %q: en=%v zh=%v", key, enPH, zhPH)
				break
			}
		}
	}

	for key := range zhMap {
		if _, ok := enMap[key]; !ok {
			t.Errorf("key %q present in zh.yaml but missing in en.yaml", key)
		}
	}
}

func TestLocalizedErrorResolution(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("i18n.Init() failed: %v", err)
	}

	inner := NewError(MsgUserTransferMinQuota, map[string]any{"Min": "$1.00"})
	outer := NewError(MsgUserTransferFailed, map[string]any{"Error": inner})

	if got := ResolveError(LangEn, outer, ""); got != "Transfer failed: Minimum transfer amount is $1.00" {
		t.Fatalf("unexpected EN localized error: %q", got)
	}
	if got := ResolveError(LangZh, outer, ""); got != "划转失败：划转额度最小为 $1.00" {
		t.Fatalf("unexpected ZH localized error: %q", got)
	}

	withFallback := NewErrorWithFallback("legacy 不是有效字段", MsgChannelSettingInvalidField, map[string]any{
		"Field": "foo",
		"Scope": "setting",
	})
	if withFallback.Error() != "legacy 不是有效字段" {
		t.Fatalf("unexpected Fallback Error(): %q", withFallback.Error())
	}
	if got := ResolveError(LangEn, withFallback, ""); got != "\"foo\" is not a valid channel setting field" {
		t.Fatalf("unexpected EN translated fallback error: %q", got)
	}

	raw := errors.New("sql failure")
	if got := ResolveError(LangEn, raw, MsgDatabaseError); got != Translate(LangEn, MsgDatabaseError, nil) {
		t.Fatalf("unexpected non-localized error fallback: %q", got)
	}
}
