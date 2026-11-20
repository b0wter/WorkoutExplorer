// The device line (F-43 DI-7) in the page language (F-54): the owner chunk's view model and the
// viewing page's lazy language pass. Not in the activity entry.
import type { Device } from '../datafile';
import { en, t } from './device-texts';

/** F-43 DI-7: the device line, '' without a name and sensors. Codes without a label are skipped. */
export function deviceLine(device: Device): string {
  const list = device.sensors
    .filter((c) => c in en)
    .map((c) => t(c as keyof typeof en))
    .join(', ');
  if (device.name !== '') return t(list === '' ? 'recorded' : 'recordedSensors', { name: device.name, list });
  return list === '' ? '' : t('sensors', { list });
}

