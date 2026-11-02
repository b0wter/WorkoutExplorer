# F-05 Stacked charts with shared cursor

- **Phase:** 0 Prototype
- **Owner:** frontend
- **Requirements:** AP-4, AP-6 (chart side), NFR Performance (60 fps scrubbing on a 12-hour workout); Technical design: Front end (Implementation notes), Workout data file (Charts)
- **Depends on:** F-04

## Goal
Draw every present stream as stacked uPlot charts on one shared x axis, driven by a single cursor store, smooth enough to scrub a 12-hour workout on a phone.

## Scope
- Stacked charts in this order: elevation, speed (or pace, F-14), heart rate, power, cadence, temperature. Only streams present in the file are shown.
- One shared x axis: elapsed time (distance toggle comes in F-17).
- Min/max-per-pixel rendering computed in the browser; full-resolution data kept for statistics.
- Missing samples (`NaN`) and gaps (5 s or more between samples) drawn as breaks, never as zero.
- One shared cursor store holding a sample index; charts read and write it; no component talks to another directly.
- Hover (mouse) and touch-drag move the cursor; a readout shows every present value at the cursor.
- Touch: horizontal drag scrubs, vertical swipes still scroll the page (assumption, see README Questions for review).
- No per-frame allocations or framework reactivity on the scrubbing hot path.

## Out of scope
- Map (F-06); range selection and x-axis toggle (F-17); units and pace (F-14); page layout (F-13).

## Acceptance criteria
- [ ] A ride with all streams shows six charts aligned on one x axis; a running file without power shows no power chart; an indoor file without altitude shows no elevation chart.
- [ ] Hovering or touch-dragging any chart moves the cursor line in every chart and updates the readout with all present values at that sample.
- [ ] A missing heart-rate section (sensor dropout) appears as a break in the line, not a drop to zero.
- [ ] A multi-hour pause draws as a break on the time axis.
- [ ] On a phone, a vertical swipe starting on a chart scrolls the page.
- [ ] Resizing the window or rotating the phone redraws the charts at the new width.
- [ ] Scrubbing the 12-hour corpus file stays at 60 fps on a desktop browser (phone measurement in F-07).
