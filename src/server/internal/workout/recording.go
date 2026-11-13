package workout

import "time"

// Recording is ingest's library-free extraction of a FIT file and the input
// to Normalise. Plain values only: times are UTC with the zero value meaning
// missing, numbers use NaN for missing.
type Recording struct {
	FileType    string
	Device      Device
	LocalOffset *int64          // activity.local_timestamp - activity.timestamp in seconds; validated in R-9
	Sessions    []RawSession    // arrival order
	Laps        []RawLap        // arrival order
	Lengths     []RawLength     // arrival order (F-48 PL-1)
	Timer       []RawTimerEvent // arrival order
	Records     []RawRecord     // arrival order, not yet sorted
	Integrity   Integrity       // set by ingest (F-08); Normalise copies it

	Sports         []RawSport // sport messages that a record followed, arrival order (D-12)
	SequenceStarts []int      // index into Records of the first record of sequence 2, 3, ... (R-12)
}

// RawSport is a FIT sport message. At is the time of the next record in the
// same sequence, the position of the message on the time axis.
type RawSport struct {
	At    time.Time
	Sport string
}

// RawSession is a FIT session as recorded.
type RawSession struct {
	Sport, SubSport string
	Start, End      time.Time
	Totals          Totals
	Pool            Pool
	Stroke          string // F-48: a stroke code of ADR-048 item 2, "" = none
}

// RawLap is a FIT lap as recorded.
type RawLap struct {
	Sport      string // lap.sport; "" if not recorded
	Trigger    string // F-46 LS-1: button, time, distance, position, stop, equipment or ""
	Stroke     string // F-48: a stroke code of ADR-048 item 2, "" = none
	Start, End time.Time
	Totals     Totals
}

// RawLength is a FIT length as recorded (F-48 PL-1).
type RawLength struct {
	Start, End time.Time
	Rest       bool    // length_type idle
	Stroke     string  // stroke code, "" = none
	Time       float64 // total_timer_time in s; NaN = not recorded
	AvgSpeed   float64 // m/s; NaN = not recorded
	Strokes    uint16  // total_strokes; 0 = not recorded
}

// RawTimerEvent is a FIT timer event mapped to start or stop.
type RawTimerEvent struct {
	At   time.Time
	Kind TimerKind
}

// RawRecord is one FIT record in the units of the F-02 design (degrees, m,
// m/s, bpm, FIT cadence units, W, °C). NaN = not recorded.
type RawRecord struct {
	At time.Time
	// Values; NaN = not recorded.
	Lat, Lon, Altitude, Distance, Speed, HeartRate, Cadence, Power, Temperature float64
}
