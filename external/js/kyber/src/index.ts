import * as curve from "./curve";
import * as sign from "./sign";

export interface Group {
  scalarLen(): number;

  scalar(): Scalar;

  pointLen(): number;

  point(): Point;
}

export interface Point {
  equal(p2: Point): boolean;

  null(): Point;

  base(): Point;

  pick(callback?: (length: number) => Buffer): Point;

  set(p: Point): Point;

  clone(): Point;

  embedLen(): number;

  embed(data: Uint8Array, callback?: (length: number) => Buffer): Point;

  data(): Uint8Array;

  add(p1: Point, p2: Point): Point;

  sub(p1: Point, p2: Point): Point;

  neg(p: Point): Point;

  mul(s: Scalar, p?: Point): Point;

  marshalBinary(): Buffer;

  unmarshalBinary(bytes: Buffer): void;
}

export interface Scalar {
  marshalBinary(): Buffer;

  unmarshalBinary(bytes: Buffer): void;

  equal(s2: Scalar): boolean;

  set(a: Scalar): Scalar;

  clone(): Scalar;

  zero(): Scalar;

  add(a: Scalar, b: Scalar): Scalar;

  sub(a: Scalar, b: Scalar): Scalar;

  neg(a: Scalar): Scalar;

  div(a: Scalar, b: Scalar): Scalar;

  mul(s1: Scalar, b: Scalar): Scalar;

  inv(a: Scalar): Scalar;

  one(): Scalar;

  pick(callback?: (length: number) => Buffer): Scalar;

  setBytes(bytes: Buffer): Scalar;
}

export {
  curve,
  sign,
}