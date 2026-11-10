// Public surface of lib/page (F-13 design, "Module boundary"). Pure TypeScript. The whole-model
// builder is './view', imported by path: re-exported here it would join the activity entry (F-19).
export { distanceAdvances, lapsGroup, summaryItems } from './build';
export type { HeaderGroup, LapsGroup, PageGroup, PageLaps, PageSettings, PageViewModel, SummaryItem } from './model';
export type { LapsSection } from '../sections';
