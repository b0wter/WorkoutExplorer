import { describe, expect, it } from 'vitest';
import { problems } from './check-upload-entry.mjs';

const manifest = {
  'index.html': { file: 'assets/index.js', isEntry: true },
  'upload.html': { file: 'assets/upload.js', isEntry: true, css: ['assets/upload.css'] },
};

describe('problems', () => {
  it('passes a clean upload entry', () => {
    expect(problems(manifest, () => 'fetch("/api/status")')).toEqual([]);
  });
  it('flags cookie and storage APIs in the upload chunk only', () => {
    const read = (f) => (f === 'assets/upload.js' ? 'localStorage.x' : 'document.cookie');
    expect(problems(manifest, read)).toEqual(['assets/upload.js: cookie or storage API']);
  });
  it('F-53 AC 57: flags the list key in the viewing page, not in its lazy chunks', () => {
    const m = { ...manifest, 'index.html': { ...manifest['index.html'], dynamicImports: ['_m'] }, _m: { file: 'assets/mine.js' } };
    const read = (f) => (f === 'assets/upload.js' ? '' : 'k="wx.workouts"');
    expect(problems(m, read)).toEqual(['assets/index.js: the viewing page holds wx.workouts']);
  });
  it('flags a chunk shared by both entries', () => {
    const m = {
      'index.html': { file: 'assets/index.js', isEntry: true, imports: ['_s'] },
      'upload.html': { file: 'assets/upload.js', isEntry: true, imports: ['_s'] },
      _s: { file: 'assets/shared.js' },
    };
    expect(problems(m, () => '')).toEqual(['assets/shared.js: shared by upload.html and index.html']);
  });
  it('flags a missing upload entry', () => {
    expect(problems({ 'index.html': manifest['index.html'] }, () => '')).toHaveLength(1);
  });
});
