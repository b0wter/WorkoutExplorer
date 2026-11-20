// The format table (F-54 LG-3, LG-7): unit symbols that differ between languages and the expiry;
// sport names are ./dates-texts (lazy). International symbols (km, mi, m, ft, yd, W, kcal, °C, °F, km/h, mph) are the same in the
// four languages and stay in code.
import { texts } from '../i18n';

export const en = {
  bpm: 'bpm',
  rpm: 'rpm',
  spm: 'spm',
  perMin: '/min',
  expires: 'Expires {when}',
  expiresNow: 'Expires now',
  expiresSoon: 'Expires in less than a minute',
};

export const t = texts(en);
