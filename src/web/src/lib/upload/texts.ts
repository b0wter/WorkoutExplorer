// The upload page's own sentences (F-54 LG-3; F-11 U-8, F-35); server messages are lib/errors'.
import { texts } from '../i18n';

export const en = {
  tooLarge: 'This file is larger than 20 MB.',
  uploadUnreadable: 'The upload could not be read. Please try again.',
  paused: 'Uploads are paused because storage is full. Please try again later.',
  opening: 'Opening the archive…',
  noZip: 'This browser cannot open ZIP archives. Please unzip it and upload the FIT file.',
  noFit: 'This archive contains no FIT file.',
  several: 'This archive contains more than one FIT file. Please upload one at a time.',
  fitTooLarge: 'The FIT file in this archive is larger than 20 MB.',
  damaged: 'This archive could not be opened. Please unzip it and upload the FIT file.',
  uploadingPercent: 'Uploading… {n} %',
  reading: 'Reading the workout…',
};

export const t = texts(en);
