package ingest

import (
	"context"
	"time"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

// listener receives every decoded message (decoder.MesgListener) and keeps
// only what F-02 reads, converted into a workout.Recording. Every other
// message number, every unknown field and every developer field is ignored.
// A message is only valid during OnMesg, so it is converted before returning.
//
// F-08 adds the budgets (ADR-017 item 3: counted before conversion, so
// nothing is allocated for the message over the limit), the file type gate
// and the counters for the log line. One listener serves all sequences of a
// file; beginSequence starts each one.
type listener struct {
	rec workout.Recording

	cancel context.CancelCauseFunc // cancels the decode with a cause; nil = no budgets
	hook   func()                  // test hook, nil in production

	records, laps, sessions, timers, fieldDescs, sports budget
	stopped                                             bool // a budget or the type gate cancelled the decode

	// Per sequence.
	seqNo        int      // number of the current sequence, from 1
	pendingSport []string // sport messages waiting for the next record of this sequence (D-12)
	seqFileID    bool     // a file_id was seen in this sequence
	skip         bool     // this sequence is of another file type: its messages are dropped
	seqMessages  int      // messages delivered in this sequence

	recordMessages int    // record messages read, all sequences (log line)
	garminProduct  string // product name from the FIT profile (Garmin family only): log line and F-43 model fallback

	creatorName string // F-43 DI-2: the creator's device_info product name, first sequence, sanitised
}

func newListener(lim Limits, cancel context.CancelCauseFunc) *listener {
	return &listener{
		cancel:     cancel,
		records:    budget{name: limitRecords, limit: lim.MaxRecords},
		laps:       budget{name: limitLaps, limit: lim.MaxLaps},
		sessions:   budget{name: limitSessions, limit: lim.MaxSessions},
		timers:     budget{name: limitTimerEvents, limit: lim.MaxTimerEvents},
		fieldDescs: budget{name: limitFieldDescriptions, limit: lim.MaxFieldDescriptions},
		sports:     budget{name: limitSportMessages, limit: lim.MaxSportMessages},
	}
}

// beginSequence starts FIT sequence n (from 1). The file type gate depends on
// n, not on whether a file_id was seen before (D-15, QA-08-r1-4). For n >= 2
// the number of records so far is kept in SequenceStarts (R-12).
func (l *listener) beginSequence(n int) {
	l.seqNo = n
	l.seqFileID, l.skip, l.seqMessages = false, false, 0
	l.pendingSport = l.pendingSport[:0] // a sport message after a sequence's last record is dropped
	if n >= 2 {
		l.rec.SequenceStarts = append(l.rec.SequenceStarts, len(l.rec.Records))
	}
}

// stop cancels the decode with cause; from then on OnMesg returns at once.
func (l *listener) stop(cause error) {
	l.stopped = true
	if l.cancel != nil {
		l.cancel(cause)
	}
}

// over counts one message against b and stops the decode if it is over.
func (l *listener) over(b *budget) bool {
	if b.add() {
		l.stop(&limitError{b.name})
		return true
	}
	return false
}

// OnMesg implements decoder.MesgListener.
func (l *listener) OnMesg(m proto.Message) {
	if l.hook != nil {
		l.hook()
	}
	if l.stopped {
		return
	}
	l.seqMessages++
	if m.Num == mesgnum.FileId {
		l.fileID(mesgdef.NewFileId(&m))
		return
	}
	// Budgets first: counted for skipped sequences too.
	switch m.Num {
	case mesgnum.Record:
		l.recordMessages++
		if l.over(&l.records) {
			return
		}
	case mesgnum.Lap:
		if l.over(&l.laps) {
			return
		}
	case mesgnum.Session:
		if l.over(&l.sessions) {
			return
		}
	case mesgnum.Event:
		ev := mesgdef.NewEvent(&m)
		if ev.Event == typedef.EventTimer && l.over(&l.timers) {
			return
		}
		if !l.skip {
			l.event(ev)
		}
		return
	case mesgnum.FieldDescription:
		if l.over(&l.fieldDescs) {
			return
		}
	case mesgnum.Sport:
		if l.over(&l.sports) {
			return
		}
	}
	if l.skip {
		return
	}
	switch m.Num {
	case mesgnum.Activity:
		l.activity(mesgdef.NewActivity(&m))
	case mesgnum.Session:
		l.session(mesgdef.NewSession(&m))
	case mesgnum.Lap:
		l.lap(mesgdef.NewLap(&m))
	case mesgnum.Record:
		l.record(mesgdef.NewRecord(&m))
	case mesgnum.Sport:
		l.sport(mesgdef.NewSport(&m))
	case mesgnum.DeviceInfo:
		l.deviceInfo(&m)
	}
}

// fileID applies the file type gate and reads type, manufacturer, product
// and product name; never the serial number. Only the first file_id of the
// first sequence sets FileType and Device and can reject the file (D-15,
// ADR-018 item 1): the file is judged by its records if that sequence has no
// file_id. A later file_id in the same sequence is ignored (E-4). In a later
// sequence, a valid type other than activity marks the sequence as skipped;
// its fields are never read.
func (l *listener) fileID(f *mesgdef.FileId) {
	if l.seqFileID {
		return
	}
	l.seqFileID = true
	otherType := f.Type != typedef.FileInvalid && f.Type != typedef.FileActivity
	if l.seqNo >= 2 {
		l.skip = otherType
		return
	}
	l.rec.FileType = enumName(f.Type.String(), uint64(f.Type), uint64(typedef.FileInvalid))
	l.rec.Device.Manufacturer = enumName(f.Manufacturer.String(), uint64(f.Manufacturer), uint64(typedef.ManufacturerInvalid))
	if f.Product != basetype.Uint16Invalid {
		l.rec.Device.ProductID = f.Product
	}
	l.rec.Device.ProductName = f.ProductName
	l.garminProduct = garminProductName(f)
	l.rec.Device.Brand = readable(l.rec.Device.Manufacturer)
	if otherType {
		l.stop(errUnsupportedType)
	}
}

// sport keeps the sport of a sport message until the next record of the same
// sequence gives it a time (D-12).
func (l *listener) sport(s *mesgdef.Sport) {
	name := enumName(s.Sport.String(), uint64(s.Sport), uint64(typedef.SportInvalid))
	l.pendingSport = append(l.pendingSport, name)
}

// activity reads the local time offset; the last activity message wins.
func (l *listener) activity(a *mesgdef.Activity) {
	l.rec.LocalOffset = localOffset(dateTime(a.Timestamp), dateTime(a.LocalTimestamp))
}

func (l *listener) session(s *mesgdef.Session) {
	start, end := span(s.StartTime, s.Timestamp, s.TotalElapsedTime)
	l.rec.Sessions = append(l.rec.Sessions, workout.RawSession{
		Sport:    enumName(s.Sport.String(), uint64(s.Sport), uint64(typedef.SportInvalid)),
		SubSport: enumName(s.SubSport.String(), uint64(s.SubSport), uint64(typedef.SubSportInvalid)),
		Start:    start,
		End:      end,
		Totals: totals(rawTotals{
			elapsed: s.TotalElapsedTime, timer: s.TotalTimerTime, distance: s.TotalDistance,
			ascent: s.TotalAscent, descent: s.TotalDescent, calories: s.TotalCalories,
			avgSpeed: s.AvgSpeed, maxSpeed: s.MaxSpeed,
			enhancedAvgSpeed: s.EnhancedAvgSpeed, enhancedMaxSpeed: s.EnhancedMaxSpeed,
			avgHR: s.AvgHeartRate, maxHR: s.MaxHeartRate,
			avgCadence: s.AvgCadence, maxCadence: s.MaxCadence,
			avgFractional: s.AvgFractionalCadence, maxFractional: s.MaxFractionalCadence,
			avgPower: s.AvgPower, maxPower: s.MaxPower,
			avgTemp: s.AvgTemperature, maxTemp: s.MaxTemperature,
		}),
		Pool: pool(s.PoolLength, s.PoolLengthUnit, s.NumActiveLengths),
	})
}

func (l *listener) lap(s *mesgdef.Lap) {
	start, end := span(s.StartTime, s.Timestamp, s.TotalElapsedTime)
	l.rec.Laps = append(l.rec.Laps, workout.RawLap{
		Sport: enumName(s.Sport.String(), uint64(s.Sport), uint64(typedef.SportInvalid)),
		Start: start,
		End:   end,
		Totals: totals(rawTotals{
			elapsed: s.TotalElapsedTime, timer: s.TotalTimerTime, distance: s.TotalDistance,
			ascent: s.TotalAscent, descent: s.TotalDescent, calories: s.TotalCalories,
			avgSpeed: s.AvgSpeed, maxSpeed: s.MaxSpeed,
			enhancedAvgSpeed: s.EnhancedAvgSpeed, enhancedMaxSpeed: s.EnhancedMaxSpeed,
			avgHR: s.AvgHeartRate, maxHR: s.MaxHeartRate,
			avgCadence: s.AvgCadence, maxCadence: s.MaxCadence,
			avgFractional: s.AvgFractionalCadence, maxFractional: s.MaxFractionalCadence,
			avgPower: s.AvgPower, maxPower: s.MaxPower,
			avgTemp: s.AvgTemperature, maxTemp: s.MaxTemperature,
		}),
	})
}

// span completes a session's or lap's start and end from start_time,
// timestamp and total_elapsed_time (conversion table rows; E-2), in UTC.
func span(startTime, timestamp time.Time, elapsedRaw uint32) (time.Time, time.Time) {
	startTime, timestamp = dateTime(startTime), dateTime(timestamp)
	end := spanEnd(startTime, timestamp, elapsedRaw)
	return utc(spanStart(startTime, end, elapsedRaw)), utc(end)
}

// record converts one record; records without a valid timestamp (E-8) are dropped.
func (l *listener) record(r *mesgdef.Record) {
	at := dateTime(r.Timestamp)
	if at.IsZero() {
		return
	}
	for _, name := range l.pendingSport {
		l.rec.Sports = append(l.rec.Sports, workout.RawSport{At: at, Sport: name})
	}
	l.pendingSport = l.pendingSport[:0]
	lat, lon := position(r.PositionLat, r.PositionLong)
	l.rec.Records = append(l.rec.Records, workout.RawRecord{
		At:          at,
		Lat:         lat,
		Lon:         lon,
		Altitude:    altitude(r.Altitude, r.EnhancedAltitude),
		Distance:    distance(r.Distance),
		Speed:       speed(r.Speed, r.EnhancedSpeed),
		HeartRate:   uint8Value(r.HeartRate),
		Cadence:     cadence(r.Cadence, r.FractionalCadence),
		Power:       uint16Value(r.Power),
		Temperature: sint8Value(r.Temperature),
	})
}

// event keeps timer events only: start → start; stop, stop_all,
// stop_disable, stop_disable_all → stop; other types are dropped.
func (l *listener) event(e *mesgdef.Event) {
	at := dateTime(e.Timestamp)
	if e.Event != typedef.EventTimer || at.IsZero() {
		return
	}
	kind, ok := timerKind(e.EventType)
	if !ok {
		return
	}
	l.rec.Timer = append(l.rec.Timer, workout.RawTimerEvent{At: at, Kind: kind})
}

func timerKind(t typedef.EventType) (workout.TimerKind, bool) {
	switch t {
	case typedef.EventTypeStart:
		return workout.TimerStart, true
	case typedef.EventTypeStop, typedef.EventTypeStopAll, typedef.EventTypeStopDisable, typedef.EventTypeStopDisableAll:
		return workout.TimerStop, true
	}
	return 0, false
}
