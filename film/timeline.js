// Scene windows in seconds. 120 BPM: one bar = 2 s, so cuts land on the beat.
export const BPM = 120;
export const DURATION = 96;
export const FPS = 60;

export const T = {
  cold: [0, 6],
  again: [6, 11],
  scattered: [11, 16],
  reveal: [16, 22],
  create: [22, 32],
  find: [32, 42],
  connect: [42, 50],
  update: [50, 60],
  check: [60, 70],
  ship: [70, 78],
  loop: [78, 86],
  cta: [86, 96],
};

export const CHAPTERS = [
  ['create', '01', 'Create'],
  ['find', '02', 'Find'],
  ['connect', '03', 'Connect'],
  ['update', '04', 'Update'],
  ['check', '05', 'Check'],
  ['ship', '06', 'Publish'],
];
