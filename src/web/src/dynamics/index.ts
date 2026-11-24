// The running dynamics chunk (F-68 XS-9). LAZY: the page glue imports this, with its text group, after
// the data file is decoded and only when the file has one of the four streams (the entry knows no
// label, unit, colour or range rule of them). Importing it installs the spec list and the range
// panel's extra cells (lib/charts `dynamics`).

import './dynamics.css';
import { dynamics } from '../lib/charts';
import { rangeCells } from './range';
import { dynamicsSpecs } from './specs';

dynamics.specs = dynamicsSpecs;
dynamics.range = rangeCells;
