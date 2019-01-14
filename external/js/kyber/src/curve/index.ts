import nist from "./nist"
import edwards25519 from "./edwards25519"
import { Group } from "..";

const mappings = {};
mappings["edwards25519"] = edwards25519.Curve;
mappings["p256"] = nist.Curve.bind(nist.Params.p256);

/**
 * availableCurves returns all the curves currently implemented as an array of string
 */
export function availableCurves() {
  return Object.keys(mappings);
}

/**
 * newCurve returns a new curve from its name. The name must be in the list returned by `availableCurves()`.
 * @throws {Error} if the name is not known.
 */
export function newCurve(name: string): Group {
  if (!(name in mappings)) throw new Error("curve not known");
  return new mappings[name]();
}

export {
    nist,
    edwards25519
}