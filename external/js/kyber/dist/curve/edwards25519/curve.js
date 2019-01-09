"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (Object.hasOwnProperty.call(mod, k)) result[k] = mod[k];
    result["default"] = mod;
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
const point_1 = __importDefault(require("./point"));
const scalar_1 = __importDefault(require("./scalar"));
const crypto = __importStar(require("crypto"));
const elliptic_1 = require("elliptic");
const BN = require("bn.js");
const ec = new elliptic_1.eddsa("ed25519");
const orderRed = BN.red(ec.curve.n);
class Ed25519 {
    constructor() {
        this.curve = ec.curve;
    }
    /**
     * Return the name of the curve
     */
    string() {
        return "Ed25519";
    }
    /**
     * Returns 32, the size in bytes of a Scalar on Ed25519 curve
     */
    scalarLen() {
        return 32;
    }
    /**
     * Returns a new Scalar for the prime-order subgroup of Ed25519 curve
     */
    scalar() {
        return new scalar_1.default(this, orderRed);
    }
    /**
     * Returns 32, the size of a Point on Ed25519 curve
     *
     * @returns {number}
     */
    pointLen() {
        return 32;
    }
    /**
     * Creates a new point on the Ed25519 curve
     */
    point() {
        return new point_1.default(this);
    }
    /**
     * NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
     * it to be a multiple of 8).
     */
    newKey() {
        let bytes = crypto.randomBytes(32);
        let hash = crypto.createHash("sha512");
        hash.update(bytes);
        let scalar = Buffer.from(hash.digest());
        scalar[0] &= 0xf8;
        scalar[31] &= 0x3f;
        scalar[31] |= 0x40;
        return this.scalar().setBytes(scalar);
    }
}
exports.default = Ed25519;
