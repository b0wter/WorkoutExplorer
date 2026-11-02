// Package workout holds the normalised workout and every normalisation rule
// (architect/F-02-fit-parsing-and-normalisation.md, rules R-1 to R-10).
//
// The package is pure: standard library only, no file or network I/O, no
// logging (Technical Design, Code structure: Dependency rules). Its input is
// a Recording, the library-free extraction of a FIT file made by package
// ingest.
package workout

import "time"

// Workout is the normalised workout: sessions, laps, timer events and
// streams on one time axis (Technical Design, Glossary "Normalised workout").
type Workout struct {
	FileType  string // FIT file_id.type name, e.g. "activity"; "" if not recorded
	Device    Device
	Sport     string    // R-9: the session's sport, "multisport" for 2+ sessions, "" without sessions
	StartTime time.Time // UTC; t = 0 of the time axis (R-1); zero if the file has no valid time
	TimeZone  TimeZone  // R-9
	Sessions  []Session // file order
	Laps      []Lap     // file order
	Timer     []TimerEvent
	Samples   Samples
	Derived   Derived
	Summary   Summary
	Integrity Integrity
}

// Device identifies the recording device. The serial number is never read
// (store as little as possible).
type Device struct {
	Manufacturer string // FIT profile name, e.g. "garmin"; "" if not recorded
	ProductID    uint16 // 0 = absent
	ProductName  string
}

// TimeZone is a fixed UTC offset for the whole workout (decision 13).
// Known=false means the offset is unknown and times are shown as UTC.
type TimeZone struct {
	Known         bool
	OffsetSeconds int32
}

// Session is a FIT session with its device totals; Start and End are time
// offsets in seconds.
type Session struct {
	Sport    string
	SubSport string
	Start    int32
	End      int32
	Totals   Totals
}

// Lap is a FIT lap; Start and End are time offsets in seconds. Its Totals
// are the device's values where recorded; Normalise fills the AP-8 columns
// the device left out from the samples, as for a range (F-18 L-1).
type Lap struct {
	Start  int32
	End    int32
	Totals Totals
}

// TimerKind is the kind of a timer event.
type TimerKind int

// Timer event kinds.
const (
	TimerStart TimerKind = iota + 1
	TimerStop
)

// TimerEvent is a timer start or stop at a time offset in seconds.
type TimerEvent struct {
	Offset int32
	Kind   TimerKind
}

// Totals are device values as recorded; nil means not recorded (AP-2).
// Units: see the F-02 design, section Units.
type Totals struct {
	ElapsedTime, TimerTime, Distance, Ascent, Descent, Calories,
	AvgSpeed, MaxSpeed, AvgHeartRate, MaxHeartRate, AvgCadence, MaxCadence,
	AvgPower, MaxPower, AvgTemperature, MaxTemperature *float64
}

// Samples are the streams on the shared time axis. A stream is nil when it
// has no valid value (AP-4); NaN marks a missing sample in a present stream.
type Samples struct {
	// Offset holds seconds since StartTime, strictly increasing.
	Offset []int32
	// Streams, each nil or of len(Offset).
	Lat, Lon, Altitude, Distance, Speed, HeartRate, Cadence, Power, Temperature []float64
	// MovingTime and Gain are the cumulative series of F-17 B-1 (ADR-029),
	// set by Normalise after gap filling: nil when the total is absent,
	// otherwise of len(Offset), non-decreasing, starting at 0 (ADR-029 rev. 1),
	// ending at Derived.MovingTime
	// and Derived.ElevationGain. Seconds and metres. Never public-transformed.
	MovingTime, Gain []float64
	// Filled counts the samples inserted by gap filling (R-3).
	Filled int
	// inserted marks the samples gap filling inserted (nil: none), so that
	// the start and end hiding measures over recorded fixes only (F-16
	// H-3). Never encoded.
	inserted []bool
}

// Inserted reports whether sample i was inserted by gap filling (R-3).
func (s Samples) Inserted(i int) bool { return s.inserted != nil && s.inserted[i] }

// Moving time sources (Derived.MovingTimeSource).
const (
	movingTimeFromTimer = "timer"
	movingTimeFromSpeed = "speed"
)

// Derived values are computed from the samples.
type Derived struct {
	ElapsedTime      *float64 // last offset; nil if no samples
	MovingTime       *float64 // R-7
	MovingTimeSource string   // "timer", "speed" or "" (MovingTime nil)
	ElevationGain    *float64 // R-8
	Distance         *float64 // last valid value of the distance stream
	ElevationLoss    *float64 // R-8 (F-08): confirmed drops of the same zig-zag pass

	// Record totals (F-08 R-11), computed from the final samples; nil when
	// the stream is absent.
	AvgSpeed, MaxSpeed             *float64
	AvgHeartRate, MaxHeartRate     *float64
	AvgCadence, MaxCadence         *float64
	AvgPower, MaxPower             *float64
	AvgTemperature, MaxTemperature *float64
}

// Integrity says how completely the file could be read (F-08; UP-2, UP-3).
type Integrity struct {
	Truncated        bool  // the file broke off; data after DataEnd is missing (UP-3 notice)
	DataEnd          int32 // time offset of the last intact record (s); set by Normalise, only if Truncated
	CRCFailed        bool  // a CRC check failed or the CRC was missing; the data was used as read
	Sequences        int   // FIT sequences found (> 1: chained file)
	SkippedSequences int   // later sequences of another file type, ignored
	IgnoredBytes     int   // trailing bytes that were not a FIT sequence
}

// Summary is what the activity page shows as the workout's numbers (R-10).
type Summary struct {
	Totals
	MovingTime *float64
}
