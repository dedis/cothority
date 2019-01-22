import nist from "./nist";
import edwards25519 from "./edwards25519";
import { Group } from "..";
/**
 * availableCurves returns all the curves currently implemented as an array of string
 */
export declare function availableCurves(): string[];
/**
 * newCurve returns a new curve from its name. The name must be in the list returned by `availableCurves()`.
 * @throws {Error} if the name is not known.
 */
export declare function newCurve(name: string): Group;
export { nist, edwards25519 };
