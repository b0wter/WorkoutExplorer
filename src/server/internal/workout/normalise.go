package workout

// Normalise turns a Recording into a normalised workout by applying the
// rules R-1 to R-14 of the F-02 and F-08 designs in order. It is deterministic and
// never panics, also for an empty Recording.
func Normalise(rec Recording) Workout {
	w := Workout{
		FileType: rec.FileType,
		Device:   rec.Device,
	}

	// R-1 start time and offsets of sessions, laps and timer events.
	w.StartTime = startTime(rec)
	w.Sessions = make([]Session, 0, len(rec.Sessions))
	for _, s := range rec.Sessions {
		start, end := spanOffsets(w.StartTime, s.Start, s.End)
		w.Sessions = append(w.Sessions, Session{
			Sport: s.Sport, SubSport: s.SubSport, Start: start, End: end, Totals: s.Totals, Pool: s.Pool,
		})
	}
	w.Laps = make([]Lap, 0, len(rec.Laps))
	for _, l := range rec.Laps {
		start, end := spanOffsets(w.StartTime, l.Start, l.End)
		w.Laps = append(w.Laps, Lap{Start: start, End: end, Totals: l.Totals})
	}
	w.Timer = make([]TimerEvent, 0, len(rec.Timer))
	for _, e := range rec.Timer {
		if e.At.IsZero() || (e.Kind != TimerStart && e.Kind != TimerStop) {
			continue
		}
		w.Timer = append(w.Timer, TimerEvent{Offset: offsetOf(w.StartTime, e.At), Kind: e.Kind})
	}

	// R-2 time axis, R-3 gap filling, R-4 stream presence.
	// R-12 first: the distance continues across FIT sequences.
	w.Samples = buildTimeAxis(continueDistance(rec.Records, rec.SequenceStarts), w.StartTime)
	fillGaps(&w.Samples)
	// R-14 (D-25, QA-08-r3-2): the device distance never falls; right after R-3.
	holdDistance(&w.Samples)

	// The sport timeline needs no samples (D-12), so it is built here for R-13.
	tl := newSportTimeline(w.Sessions, collectHints(rec, w.StartTime))

	// R-13 transition legs (D-21, D-22): after R-3, before R-4.
	maskTransitions(&w, tl)
	settlePresence(&w.Samples)

	// Integrity first: the record leg of the summary needs Truncated.
	w.Integrity = rec.Integrity

	// R-5 cadence by sport, through the sport timeline (D-12).
	convertCadence(&w, tl)

	// R-6 distance.
	computeDistance(&w.Samples)
	w.Derived.Distance = lastValid(w.Samples.Distance)

	// R-7 moving time, R-8 elevation gain.
	if n := len(w.Samples.Offset); n > 0 {
		e := float64(w.Samples.Offset[n-1])
		w.Derived.ElapsedTime = &e
	}
	w.Derived.MovingTime, w.Derived.MovingTimeSource, w.Samples.MovingTime = movingTime(w.Timer, w.Samples, tl)
	w.Derived.ElevationGain, w.Derived.ElevationLoss, w.Samples.Gain = elevationCum(w.Samples.Offset, w.Samples.Altitude, hysteresisMetres)

	// R-11 record totals (F-08), needed by the summary fallbacks.
	recordTotals(&w)

	// R-9 time zone and sport; R-10 summary over the legs.
	w.TimeZone = timeZone(rec.LocalOffset)
	legs, sports := legsOf(&w, tl)
	w.Sport = workoutSport(sports)
	// R-10.E (D-18, D-20): the device's elapsed time survives only for one
	// session that is the whole recording, in one FIT sequence.
	single := len(w.Sessions) == 1 && len(legs) == 1 && recordSequences(rec) <= 1
	w.Summary = summarise(legs, sports, w.Derived, single)
	w.Summary.PoolLengthM, w.Summary.PoolLengthYd, w.Summary.Lengths = poolSummary(w.Sessions)

	// F-18 L-1: lap values the device did not record, as for a range.
	completeLaps(&w, tl)

	// F-08 integrity: the streams end at the last intact record.
	w.Integrity.DataEnd = 0
	if w.Integrity.Truncated {
		if n := len(w.Samples.Offset); n > 0 {
			w.Integrity.DataEnd = w.Samples.Offset[n-1]
		}
	}
	return w
}
