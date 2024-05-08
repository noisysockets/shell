import { TextEncoder, TextDecoder } from 'util'

Object.defineProperty(global, 'TextEncoder', {
  value: TextEncoder,
});

Object.defineProperty(global, 'TextDecoder', {
  value: TextDecoder,
});