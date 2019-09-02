import nist from "./nist"
import edwards25519 from "./edwards25519"
import { Group } from "..";

function p256Group(): Group {
    return new nist.Curve(nist.Params.p256);
}

const mappings: Map<string,() => Group> = new Map([
    ["edwards25519", () => new edwards25519.Curve()],
    ["p256", p256Group],
]);

/**
 * availableCurves returns all the curves currently implemented as an array of string
 */
export function availableCurves(): string[] {
    return Array.from(mappings.keys());
}

/**
 * newCurve returns a new curve from its name. The name must be in the list returned by `availableCurves()`.
 * @throws {Error} if the name is not known.
 */
export function newCurve(name: string): Group {
    const got = mappings.get(name);
    if (got === undefined) throw new Error("curve not known");
    return got();
}

export {
    nist,
    edwards25519
}
