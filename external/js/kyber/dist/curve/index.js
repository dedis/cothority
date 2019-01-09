"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const nist_1 = __importDefault(require("./nist"));
exports.nist = nist_1.default;
const edwards25519_1 = __importDefault(require("./edwards25519"));
exports.edwards25519 = edwards25519_1.default;
const mappings = {};
mappings["edwards25519"] = edwards25519_1.default.Curve;
mappings["p256"] = nist_1.default.Curve.bind(nist_1.default.Params.p256);
/**
 * availableCurves returns all the curves currently implemented as an array of string
 */
function availableCurves() {
    return Object.keys(mappings);
}
exports.availableCurves = availableCurves;
/**
 * newCurve returns a new curve from its name. The name must be in the list returned by `availableCurves()`.
 * @throws {Error} if the name is not known.
 */
function newCurve(name) {
    if (!(name in mappings))
        throw new Error("curve not known");
    return new mappings[name]();
}
exports.newCurve = newCurve;
