// SUMMARY.md of a Playwright JSON report (called by report.sh). Exit 1 if any test failed or flaked-out.
// usage: node summary.mjs <report.json> <commit> <ref> <start> <end> <seconds> <report dir>
import fs from 'node:fs';

const [file, commit, ref, start, end, seconds, dir] = process.argv.slice(2);
const report = JSON.parse(fs.readFileSync(file, 'utf8'));
const per = {};
const failures = [];
const flaky = [];

function walk(suite, titles) {
  const t = suite.title && !suite.file ? [...titles, suite.title] : titles;
  for (const spec of suite.specs ?? []) {
    for (const test of spec.tests ?? []) {
      const p = (per[test.projectName] ??= { passed: 0, failed: 0, flaky: 0, skipped: 0, ms: 0 });
      const name = [...t, spec.title].join(' > ');
      const last = test.results[test.results.length - 1];
      const line = (last?.error?.message ?? '').replace(/\u001b\[[0-9;]*m/g, '').split('\n').find((l) => l.trim()) ?? '';
      p.ms += test.results.reduce((a, r) => a + (r.duration ?? 0), 0);
      if (test.status === 'expected') p.passed++;
      else if (test.status === 'skipped') p.skipped++;
      else if (test.status === 'flaky') { p.flaky++; flaky.push({ name, project: test.projectName, line }); }
      else { p.failed++; failures.push({ name, project: test.projectName, line, file: spec.file }); }
    }
  }
  for (const s of suite.suites ?? []) walk(s, t);
}
for (const s of report.suites ?? []) walk(s, []);

const sum = (k) => Object.values(per).reduce((a, p) => a + p[k], 0);
const mins = `${Math.floor(seconds / 60)} min ${seconds % 60} s`;
console.log(`${sum('failed') ? 'FAILED' : 'PASSED'}: ${sum('passed')} passed, ${sum('failed')} failed, ${sum('flaky')} flaky, ${sum('skipped')} skipped`);
console.log('');
console.log(`- Commit: ${commit} (${ref})`);
console.log(`- Start: ${start}, end: ${end}, duration: ${mins}`);
console.log(`- Report: ${dir}/html/index.html (open with: npx playwright show-report ${dir}/html)`);
console.log(`- Traces and screenshots of failed tests: ${dir}/artifacts/ (trace.zip: npx playwright show-trace <file>)`);
console.log(`- Raw log: ${dir}/run.log`);
console.log('');
console.log('| Project | Passed | Failed | Flaky | Skipped | Test time (summed over workers) |');
console.log('| --- | --- | --- | --- | --- | --- |');
for (const [name, p] of Object.entries(per)) console.log(`| ${name} | ${p.passed} | ${p.failed} | ${p.flaky} | ${p.skipped} | ${Math.round(p.ms / 1000)} s |`);
if (failures.length) {
  console.log('\n## Failures\n');
  for (const f of failures) console.log(`- [${f.project}] ${f.name}: ${f.line.slice(0, 200)}`);
}
if (flaky.length) {
  console.log('\n## Flaky (passed on retry)\n');
  for (const f of flaky) console.log(`- [${f.project}] ${f.name}: ${f.line.slice(0, 200)}`);
}
process.exit(failures.length ? 1 : 0);
