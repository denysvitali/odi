package logutils

import (
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColoredFormatter_Default(t *testing.T) {
	f := &ColoredFormatter{DisableColors: true}
	ts := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	entry := &logrus.Entry{
		Time:    ts,
		Level:   logrus.InfoLevel,
		Message: "hello",
		Data:    logrus.Fields{},
	}
	out, err := f.Format(entry)
	require.NoError(t, err)
	got := string(out)
	assert.Contains(t, got, "2024-06-15 12:30:45")
	assert.Contains(t, got, "INFO")
	assert.Contains(t, got, "hello")
	assert.True(t, strings.HasSuffix(got, "\n"))
}

func TestColoredFormatter_AllLevels(t *testing.T) {
	f := &ColoredFormatter{DisableColors: true, DisableTimestamp: true}
	levels := []struct {
		level logrus.Level
		want  string
	}{
		{logrus.TraceLevel, "TRACE"},
		{logrus.DebugLevel, "DEBUG"},
		{logrus.InfoLevel, "INFO"},
		{logrus.WarnLevel, "WARN"},
		{logrus.ErrorLevel, "ERROR"},
		// Skip Fatal/Panic — entry produces them but they're handled the same.
	}
	for _, tc := range levels {
		t.Run(tc.want, func(t *testing.T) {
			entry := &logrus.Entry{
				Time:    time.Now(),
				Level:   tc.level,
				Message: "msg",
				Data:    logrus.Fields{},
			}
			out, err := f.Format(entry)
			require.NoError(t, err)
			assert.Contains(t, string(out), tc.want)
		})
	}
}

func TestColoredFormatter_PackageField(t *testing.T) {
	f := &ColoredFormatter{DisableColors: true, DisableTimestamp: true}
	entry := &logrus.Entry{
		Time:    time.Now(),
		Level:   logrus.InfoLevel,
		Message: "msg",
		Data:    logrus.Fields{"package": "indexer"},
	}
	out, err := f.Format(entry)
	require.NoError(t, err)
	assert.Contains(t, string(out), "[indexer]")
}

func TestColoredFormatter_AdditionalFields(t *testing.T) {
	f := &ColoredFormatter{DisableColors: true, DisableTimestamp: true}
	entry := &logrus.Entry{
		Time:    time.Now(),
		Level:   logrus.InfoLevel,
		Message: "msg",
		Data:    logrus.Fields{"package": "x", "user": "alice", "count": 3},
	}
	out, err := f.Format(entry)
	require.NoError(t, err)
	got := string(out)
	assert.Contains(t, got, "[x]")
	assert.Contains(t, got, "user=alice")
	assert.Contains(t, got, "count=3")
}

func TestColoredFormatter_DisableTimestamp(t *testing.T) {
	f := &ColoredFormatter{DisableColors: true, DisableTimestamp: true}
	ts := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	entry := &logrus.Entry{
		Time:    ts,
		Level:   logrus.InfoLevel,
		Message: "hello",
		Data:    logrus.Fields{},
	}
	out, err := f.Format(entry)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "2024-06-15")
}

func TestColoredFormatter_CustomTimestampFormat(t *testing.T) {
	f := &ColoredFormatter{DisableColors: true, TimestampFormat: "2006/01/02"}
	ts := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	entry := &logrus.Entry{
		Time:    ts,
		Level:   logrus.InfoLevel,
		Message: "hello",
		Data:    logrus.Fields{},
	}
	out, err := f.Format(entry)
	require.NoError(t, err)
	assert.Contains(t, string(out), "2024/06/15")
}

func TestSetLoggerLevel(t *testing.T) {
	original := log.GetLevel()
	t.Cleanup(func() { log.SetLevel(original) })

	SetLoggerLevel("debug")
	assert.Equal(t, logrus.DebugLevel, log.GetLevel())

	SetLoggerLevel("warn")
	assert.Equal(t, logrus.WarnLevel, log.GetLevel())

	// Invalid → falls back to info
	SetLoggerLevel("not-a-level")
	assert.Equal(t, logrus.InfoLevel, log.GetLevel())
}

func TestSetupLogger(t *testing.T) {
	original := log.GetLevel()
	originalFmt := log.Formatter
	t.Cleanup(func() {
		log.SetLevel(original)
		log.SetFormatter(originalFmt)
	})

	SetupLogger("error")
	assert.Equal(t, logrus.ErrorLevel, log.GetLevel())
	_, ok := log.Formatter.(*ColoredFormatter)
	assert.True(t, ok, "formatter should be ColoredFormatter after SetupLogger")
}

func TestIsTerminal(t *testing.T) {
	// We can't reliably control stdout's terminal-ness in tests, but the
	// function must at minimum return a bool without panicking. In `go test`
	// stdout is typically a pipe, so this should return false.
	_ = isTerminal()
}
