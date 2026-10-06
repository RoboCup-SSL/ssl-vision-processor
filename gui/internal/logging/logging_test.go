package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]struct {
		want    slog.Level
		wantErr bool
	}{
		"debug":   {want: slog.LevelDebug},
		"Info":    {want: slog.LevelInfo},
		"  WARN ": {want: slog.LevelWarn},
		"error":   {want: slog.LevelError},
		"":        {want: slog.LevelInfo},
		"verbose": {want: slog.LevelInfo, wantErr: true},
	}

	for name, want := range cases {
		got, err := ParseLevel(name)
		if got != want.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", name, got, want.want)
		}

		if (err != nil) != want.wantErr {
			t.Errorf("ParseLevel(%q) err = %v, want error: %v", name, err, want.wantErr)
		}
	}
}

// The point of the fan-out is that each destination keeps its own format, which
// is exactly what a single io.MultiWriter cannot do.
func TestMultiHandlerKeepsPerHandlerFormat(t *testing.T) {
	var text, json bytes.Buffer

	handler := multiHandler{
		slog.NewTextHandler(&text, nil),
		slog.NewJSONHandler(&json, nil),
	}
	slog.New(handler).With("instance", "cam0").Info("announce", "camId", 3)

	if !strings.Contains(text.String(), `msg=announce instance=cam0 camId=3`) {
		t.Errorf("text handler got %q", text.String())
	}

	if !strings.Contains(json.String(), `"msg":"announce","instance":"cam0","camId":3`) {
		t.Errorf("json handler got %q", json.String())
	}
}

func TestMultiHandlerSkipsDisabledHandlers(t *testing.T) {
	var debug, errorsOnly bytes.Buffer

	handler := multiHandler{
		slog.NewTextHandler(&debug, &slog.HandlerOptions{Level: slog.LevelDebug}),
		slog.NewTextHandler(&errorsOnly, &slog.HandlerOptions{Level: slog.LevelError}),
	}
	slog.New(handler).Info("only the debug handler wants this")

	if debug.Len() == 0 {
		t.Error("debug handler received nothing")
	}

	if errorsOnly.Len() != 0 {
		t.Errorf("error-only handler received %q", errorsOnly.String())
	}
}
