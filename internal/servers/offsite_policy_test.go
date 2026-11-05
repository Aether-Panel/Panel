package servers

import (
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
	"github.com/stretchr/testify/assert"
)

func TestOffsiteDue(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Skip("no tzdata")
	}
	sat := time.Date(2026, 11, 7, 3, 0, 0, 0, loc) // Saturday
	cases := []struct {
		f         OffsiteFrequency
		last, now time.Time
		want      bool
	}{
		{OffsiteNever, time.Time{}, sat, false},
		{OffsiteAlways, sat.Add(-time.Minute), sat, true},
		{OffsiteWeekend, time.Time{}, sat.AddDate(0, 0, -1), false}, // Friday
		{OffsiteWeekend, time.Time{}, sat, true},
		{OffsiteWeekend, sat, sat.AddDate(0, 0, 1), false}, // Sunday, already uploaded on Saturday
		{OffsiteWeekend, sat.AddDate(0, 0, -7), sat, true},
		{OffsiteWeekly, time.Date(2026, 11, 2, 1, 0, 0, 0, loc), sat, false}, // Monday of this week
		{OffsiteWeekly, time.Date(2026, 11, 1, 23, 0, 0, 0, loc), sat, true}, // previous Sunday
		{OffsiteBiweekly, sat.Add(-15*24*time.Hour + time.Hour), sat, true},  // within the 2 h margin
		{OffsiteBiweekly, sat.Add(-14 * 24 * time.Hour), sat, false},
		{OffsiteMonthly, time.Date(2026, 11, 1, 0, 30, 0, 0, loc), sat, false},
		{OffsiteMonthly, time.Date(2026, 10, 31, 23, 0, 0, 0, loc), sat, true},
	}
	for i, c := range cases {
		assert.Equal(t, c.want, offsiteDue(c.f, c.last, c.now), "case %d (%s)", i, c.f)
	}
}

func TestTaskIDFromBackupName(t *testing.T) {
	u := "0b7c7a52-3d0e-4f5b-9a55-8f7f1f2d3c4e"
	id, ok := taskIDFromBackupName("auto-a-b-" + u + ".tar.gz")
	assert.True(t, ok)
	assert.Equal(t, "a-b", id)
	id, ok = taskIDFromBackupName("auto-a-" + u + ".tar.gz")
	assert.True(t, ok)
	assert.Equal(t, "a", id)
	_, ok = taskIDFromBackupName("auto-" + u + ".tar.gz") // old copy without a task
	assert.False(t, ok)
	_, ok = taskIDFromBackupName("offsite-" + u + ".tar.gz")
	assert.False(t, ok)
	assert.Equal(t, "auto-a-", taskBackupPrefix("a"))
}

func TestParseOffsiteFrequency(t *testing.T) {
	f, err := ParseOffsiteFrequency(nil)
	assert.NoError(t, err)
	assert.Equal(t, OffsiteAlways, f)
	f, err = ParseOffsiteFrequency("")
	assert.NoError(t, err)
	assert.Equal(t, OffsiteAlways, f)
	f, err = ParseOffsiteFrequency("monthly")
	assert.NoError(t, err)
	assert.Equal(t, OffsiteMonthly, f)
	_, err = ParseOffsiteFrequency("daily")
	assert.Error(t, err)
	_, err = ParseOffsiteFrequency(5)
	assert.Error(t, err)
}

func TestOffsiteKeep(t *testing.T) {
	k, err := OffsiteKeep(nil)
	assert.NoError(t, err)
	assert.Equal(t, 4, k)
	k, err = OffsiteKeep(float64(10))
	assert.NoError(t, err)
	assert.Equal(t, 10, k)
	_, err = OffsiteKeep(0)
	assert.Error(t, err)
	_, err = OffsiteKeep(51)
	assert.Error(t, err)
}

func TestValidPeriodicHours(t *testing.T) {
	assert.True(t, ValidPeriodicHours(24))
	assert.True(t, ValidPeriodicHours(0))
	assert.False(t, ValidPeriodicHours(25))
}

func TestNewestBeyond(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var objs []offsite.Object
	for _, n := range []string{"d", "a", "f", "b", "e", "c"} { // unordered on purpose
		objs = append(objs, offsite.Object{FileName: n, ModTime: base.Add(time.Duration(n[0]-'a') * time.Hour)})
	}
	assert.Equal(t, []string{"a", "b"}, newestBeyond(objs, 4))
	assert.Empty(t, newestBeyond(objs, 6))
}
