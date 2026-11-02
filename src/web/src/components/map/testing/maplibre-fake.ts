// TEST-ONLY recording fake of 'maplibre-gl' for map-view.dom.test.ts (ADR-012 rev. 3; F-06
// design, Tests). MapLibre GL JS v6 needs WebGL2 and cannot run in jsdom.
//
// Mirrors the API of maplibre-gl 6.14.0, and only the members map-view.ts calls: LngLat,
// Map (constructor, on, once, addControl, getCanvas, getZoom, unproject, setStyle, getLayer, getSource, setPaintProperty, setPixelRatio, remove,
// touchZoomRotate.disableRotation, touchZoomRotate._tapDragZoom.disable), Marker (constructor, setLngLat, addTo, remove),
// NavigationControl, setWorkerUrl. No logic beyond recording calls and emitting the events a
// test triggers. unproject is a plain linear map of the point it receives (`recorder.view`,
// set by the test; v1.3, QA-06-r1-3) and records that point; a test may make one named call
// throw (`recorder.throwIn`). The fake builds just the DOM a test dispatches events on
// (canvas container and canvas).

export interface Recorder {
  /** Call order across the module: 'setWorkerUrl', 'new Map', 'setStyle', 'map.remove', … */
  log: string[];
  workerUrl: string | null;
  maps: FakeMap[];
  markers: FakeMarker[];
  /** unproject([x, y]) = (lng0 + x · lngPerPx, lat0 − y · latPerPx). */
  view: { lng0: number; lat0: number; lngPerPx: number; latPerPx: number };
  /** Every point passed to unproject, copied. */
  points: [number, number][];
  /** Set to make the next Map constructor throw. */
  constructorError: Error | null;
  /** Names one call that throws: 'Marker.addTo', 'Map.addControl' or 'Map.on:<type>'. */
  throwIn: string | null;
}

export const recorder: Recorder = {
  log: [],
  workerUrl: null,
  maps: [],
  markers: [],
  view: { lng0: 0, lat0: 0, lngPerPx: 1, latPerPx: 1 },
  points: [],
  constructorError: null,
  throwIn: null,
};

function maybeThrow(call: string): void {
  if (recorder.throwIn === call) throw new Error(`fake ${call} (test)`);
}

export function resetRecorder(): void {
  recorder.log = [];
  recorder.workerUrl = null;
  recorder.maps = [];
  recorder.markers = [];
  recorder.view = { lng0: 0, lat0: 0, lngPerPx: 1, latPerPx: 1 };
  recorder.points = [];
  recorder.constructorError = null;
  recorder.throwIn = null;
}

export class LngLat {
  constructor(
    public lng: number,
    public lat: number,
  ) {}
}

export function setWorkerUrl(url: string): void {
  recorder.log.push('setWorkerUrl');
  recorder.workerUrl = url;
}

type Listener = (event: { type: string }) => void;

export class FakeMap {
  readonly options: Record<string, unknown>;
  readonly listeners = new Map<string, Listener[]>();
  readonly controls: [unknown, string | undefined][] = [];
  readonly styles: [unknown, unknown][] = [];
  /** Every setPixelRatio argument, in order (F-07 D-8). */
  readonly pixelRatios: number[] = [];
  readonly canvasContainer: HTMLDivElement;
  readonly canvas: HTMLCanvasElement;
  rotationDisabled = false;
  tapDragZoomDisabled = false;
  removed = false;
  removeCount = 0;
  zoom = 14;
  readonly touchZoomRotate = {
    disableRotation: (): void => {
      this.rotationDisabled = true;
    },
    _tapDragZoom: {
      disable: (): void => {
        this.tapDragZoomDisabled = true;
      },
    },
  };

  constructor(options: Record<string, unknown>) {
    if (recorder.constructorError) throw recorder.constructorError;
    recorder.log.push('new Map');
    this.options = options;
    this.canvasContainer = document.createElement('div');
    this.canvasContainer.className = 'maplibregl-canvas-container';
    this.canvas = document.createElement('canvas');
    this.canvasContainer.appendChild(this.canvas);
    (options.container as HTMLElement).appendChild(this.canvasContainer);
    // MapLibre's attribution control: compact and open until collapsed (F-13 D-20).
    const attrib = document.createElement('details');
    attrib.className = 'maplibregl-ctrl-attrib maplibregl-compact maplibregl-compact-show';
    attrib.setAttribute('open', '');
    (options.container as HTMLElement).appendChild(attrib);
    recorder.maps.push(this);
  }

  on(type: string, listener: Listener): this {
    maybeThrow(`Map.on:${type}`);
    const list = this.listeners.get(type) ?? [];
    list.push(listener);
    this.listeners.set(type, list);
    return this;
  }

  once(type: string, listener: Listener): this {
    maybeThrow(`Map.once:${type}`);
    const wrapped: Listener = (e) => {
      const list = this.listeners.get(type) ?? [];
      this.listeners.set(
        type,
        list.filter((l) => l !== wrapped),
      );
      listener(e);
    };
    return this.on(type, wrapped);
  }

  /** Test helper: emits a map event to the registered listeners. */
  fire(type: string, extra: Record<string, unknown> = {}): void {
    for (const l of this.listeners.get(type) ?? []) l({ type, ...extra });
  }

  /** Every fitBounds call (F-13 D-20). */
  readonly fits: [unknown, unknown][] = [];
  fitBounds(bounds: unknown, options: unknown): this {
    this.fits.push([bounds, options]);
    return this;
  }

  addControl(control: unknown, position?: string): this {
    maybeThrow('Map.addControl');
    this.controls.push([control, position]);
    return this;
  }

  getCanvas(): HTMLCanvasElement {
    return this.canvas;
  }

  getZoom(): number {
    return this.zoom;
  }

  unproject(point: [number, number]): LngLat {
    recorder.log.push('unproject');
    recorder.points.push([point[0], point[1]]);
    const v = recorder.view;
    return new LngLat(v.lng0 + point[0] * v.lngPerPx, v.lat0 - point[1] * v.latPerPx);
  }

  setStyle(style: unknown, options: unknown): this {
    recorder.log.push('setStyle');
    this.styles.push([style, options]);
    return this;
  }

  /** Every setPaintProperty call as [layer, property, value] (F-14 D-12). */
  readonly paints: [string, string, unknown][] = [];
  getLayer(id: string): { id: string } | undefined {
    return id === 'background' || id.startsWith('wx-') ? { id } : undefined;
  }
  /** Every GeoJSON setData call as [source, data] (F-17 R-10); getSource knows the wx- sources. */
  readonly sourceData: [string, unknown][] = [];
  getSource(id: string): { setData(data: unknown): void } | undefined {
    if (!id.startsWith('wx-')) return undefined;
    return { setData: (data: unknown) => void this.sourceData.push([id, data]) };
  }
  setPaintProperty(layer: string, property: string, value: unknown): this {
    this.paints.push([layer, property, value]);
    return this;
  }

  setPixelRatio(pixelRatio: number): void {
    recorder.log.push('setPixelRatio');
    this.pixelRatios.push(pixelRatio);
  }

  remove(): void {
    recorder.log.push('map.remove');
    this.removeCount++;
    this.removed = true;
    this.canvasContainer.remove();
  }
}

export class FakeMarker {
  readonly options: { element: HTMLElement; anchor?: string; subpixelPositioning?: boolean };
  readonly positions: [number, number][] = [];
  map: FakeMap | null = null;
  removed = false;

  constructor(options: { element: HTMLElement; anchor?: string; subpixelPositioning?: boolean }) {
    this.options = options;
    recorder.markers.push(this);
  }

  setLngLat(at: LngLat): this {
    this.positions.push([at.lng, at.lat]);
    return this;
  }

  addTo(map: FakeMap): this {
    maybeThrow('Marker.addTo');
    this.map = map;
    map.canvasContainer.appendChild(this.options.element);
    return this;
  }

  remove(): this {
    this.removed = true;
    this.options.element.remove();
    return this;
  }
}

export class NavigationControl {
  constructor(public readonly options: unknown) {}
}

export { FakeMap as Map, FakeMarker as Marker };
