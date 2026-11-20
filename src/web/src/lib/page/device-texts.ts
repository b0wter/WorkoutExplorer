// The device line's and the expiry estimate's table (F-54 LG-3): the owner chunk and the viewing
// page's lazy language pass (src/page/header) use them, the entry not.
import { texts } from '../i18n';

export const en = {
  recorded: 'Recorded with {name}',
  recordedSensors: 'Recorded with {name} · {list}',
  sensors: 'Sensors: {list}',
  heart_rate_monitor: 'heart-rate monitor',
  power_meter: 'power meter',
  speed_sensor: 'speed sensor',
  cadence_sensor: 'cadence sensor',
  speed_cadence_sensor: 'speed and cadence sensor',
  foot_pod: 'foot pod',
  radar: 'radar',
  expiresAround: 'Expires around {date}',
};

export const t = texts(en);
