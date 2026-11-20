// The lazy map's table (F-54 LG-3, LG-7): route colouring and MapLibre's UI strings (its `locale` option).
import { texts } from '../../lib/i18n';

export const en = {
  routeColour: 'Route colour',
  plainRoute: 'Plain route',
  noData: 'no data',
  colourPace: 'Pace',
  colourSpeed: 'Speed',
  colourHeartRate: 'Heart rate',
  colourPower: 'Power',
  colourGradient: 'Gradient',
  mlAttribution: 'Toggle attribution',
  mlFullscreen: 'Enter fullscreen',
  mlExitFullscreen: 'Exit fullscreen',
  mlMap: 'Map',
  mlZoomIn: 'Zoom in',
  mlZoomOut: 'Zoom out',
  mlCtrlScroll: 'Use Ctrl + scroll to zoom the map',
  mlCmdScroll: 'Use ⌘ + scroll to zoom the map',
  mlTwoFingers: 'Use two fingers to move the map',
  /** F-61 SL-6: the satellite toggle's name and title; pressed = Satellite shown. */
  satellite: 'Satellite imagery',
};

export const t = texts(en);
/** MapLibre's `locale` option in the page language. */
export const mapLocale = (): Record<string, string> => ({
  'AttributionControl.ToggleAttribution': t('mlAttribution'),
  'FullscreenControl.Enter': t('mlFullscreen'),
  'FullscreenControl.Exit': t('mlExitFullscreen'),
  'Map.Title': t('mlMap'),
  'NavigationControl.ZoomIn': t('mlZoomIn'),
  'NavigationControl.ZoomOut': t('mlZoomOut'),
  'CooperativeGesturesHandler.WindowsHelpText': t('mlCtrlScroll'),
  'CooperativeGesturesHandler.MacHelpText': t('mlCmdScroll'),
  'CooperativeGesturesHandler.MobileHelpText': t('mlTwoFingers'),
});
