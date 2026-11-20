// @vitest-environment jsdom
// F-57 delivery (GX-7; AC 7): a blob: link clicked, revoked in a later task; no fetch, no data: URL.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { downloadGpx } from '.';

const { createObjectURL, revokeObjectURL } = URL;
afterEach(() => {
  Object.assign(URL, { createObjectURL, revokeObjectURL });
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('downloadGpx', () => {
  it('clicks a temporary anchor on a blob: URL named by the header date, revokes it later, fetches nothing', async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    let blob: Blob | null = null;
    const create = vi.fn((b: Blob) => {
      blob = b;
      return 'blob:https://example.test/1';
    });
    const revoke = vi.fn();
    Object.assign(URL, { createObjectURL: create, revokeObjectURL: revoke });
    const clicked: { href: string; download: string; attached: boolean }[] = [];
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clicked.push({ href: this.href, download: this.download, attached: this.isConnected });
    });

    downloadGpx(document, {
      latitude: Float64Array.from([NaN, 47.5, 47.6]),
      longitude: Float64Array.from([NaN, 8.5, 8.6]),
      altitude: Float64Array.from([1, NaN, 500.04]),
      startTime: '2026-10-09T22:30:00Z',
      zone: { known: true, offset_seconds: 7200 },
    });

    expect(clicked).toEqual([{ href: 'blob:https://example.test/1', download: 'route-2026-10-10.gpx', attached: false }]);
    expect(create).toHaveBeenCalledTimes(1);
    expect(blob!.type).toBe('application/gpx+xml');
    expect(await blob!.text()).toContain('<trkseg>\n<trkpt lat="47.500000" lon="8.500000"/>\n<trkpt lat="47.600000" lon="8.600000"><ele>500.0</ele></trkpt>\n</trkseg>');
    expect(revoke).not.toHaveBeenCalled(); // not in the click's task
    vi.runAllTimers();
    expect(revoke).toHaveBeenCalledWith('blob:https://example.test/1');
    expect(fetchMock).not.toHaveBeenCalled();
    expect(document.querySelector('a')).toBeNull(); // nothing left in the page
  });
});
