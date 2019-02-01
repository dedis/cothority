"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const point_1 = __importDefault(require("./point"));
const scalar_1 = __importDefault(require("./scalar"));
const crypto_1 = require("crypto");
const elliptic_1 = require("elliptic");
const bn_js_1 = __importDefault(require("bn.js"));
const ec = new elliptic_1.eddsa("ed25519");
const orderRed = bn_js_1.default.red(ec.curve.n);
class Ed25519 {
    constructor() {
        this.curve = ec.curve;
    }
    /**
     * Get the name of the curve
     * @returns the name
     */
    string() {
        return "Ed25519";
    }
    /** @inheritdoc */
    scalarLen() {
        return 32;
    }
    /** @inheritdoc */
    scalar() {
        return new scalar_1.default(this, orderRed);
    }
    /** @inheritdoc */
    pointLen() {
        return 32;
    }
    /** @inheritdoc */
    point() {
        return new point_1.default();
    }
    /**
     * NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
     * it to be a multiple of 8).
     * @returns the key as a scalar
     */
    newKey() {
        const bytes = crypto_1.randomBytes(32);
        const hash = crypto_1.createHash("sha512");
        hash.update(bytes);
        const scalar = Buffer.from(hash.digest());
        scalar[0] &= 0xf8;
        scalar[31] &= 0x3f;
        scalar[31] |= 0x40;
        return this.scalar().setBytes(scalar);
    }
}
exports.default = Ed25519;
