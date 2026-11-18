package middleware

import (
	"os"
	"testing"

	"github.com/NookMux/NookMux/internal/i18n"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
