"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const point_1 = require("./pairing/point");
const point_2 = __importDefault(require("./curve/edwards25519/point"));
/**
 * Factory that must be used to decode points coming from the network or
 * a TOML file. It will take care of looking up the right instance of the
 * point. A dedicated toProto function is provided for the points to encode
 * them back if required.
 */
class PointFactory {
    constructor() {
        this.tags = {
            [`${point_1.BN256G1Point.MARSHAL_ID.toString()}`]: () => new point_1.BN256G1Point(),
            [`${point_1.BN256G2Point.MARSHAL_ID.toString()}`]: () => new point_1.BN256G2Point(),
            [`${point_2.default.MARSHAL_ID.toString()}`]: () => new point_2.default(),
        };
        this.suites = {
            [`${PointFactory.SUITE_ED25519}`]: () => new point_2.default(),
            [`${PointFactory.SUITE_BN256}`]: () => new point_1.BN256G2Point(),
        };
    }
    /**
     * Decode a point using the 8 first bytes to look up the
     * correct instance
     *
     * @param buf bytes to decode
     * @returns the point
     */
    fromProto(buf) {
        const tag = buf.slice(0, 8).toString();
        const generator = this.tags[tag];
        if (generator) {
            const point = generator();
            point.unmarshalBinary(buf.slice(8));
            return point;
        }
        throw new Error('unknown tag for the point');
    }
    /**
     * Decode the point stored as an hexadecimal string by using
     * the suite as a reference for the point instance
     *
     * @param suite Name of the suite to use
     * @param pub   The point encoded as an hex-string
     * @returns the point
     */
    fromToml(suite, pub) {
        const generator = this.suites[suite];
        if (generator) {
            const point = generator();
            const buf = Buffer.from(pub, 'hex');
            point.unmarshalBinary(buf);
            return point;
        }
        throw new Error('unknown suite for the point');
    }
}
PointFactory.SUITE_ED25519 = 'Ed25519';
PointFactory.SUITE_BN256 = 'bn256.adapter';
exports.PointFactory = PointFactory;
exports.default = new PointFactory();
