import { Point } from ".";
import { BN256G1Point, BN256G2Point } from "./pairing/point";
import Ed25519Point from "./curve/edwards25519/point";

interface GeneratorMap {
    [k: string]: () => Point
}

/**
 * Factory that must be used to decode points coming from the network or
 * a TOML file. It will take care of looking up the right instance of the
 * point. A dedicated toProto function is provided for the points to encode
 * them back if required.
 */
export class PointFactory {
    private static SUITE_ED25519 = 'Ed25519';
    private static SUITE_BN256 = 'bn256.adapter';

    private tags: GeneratorMap;
    private suites: GeneratorMap;

    constructor() {
        this.tags = {
            [`${BN256G1Point.MARSHAL_ID.toString()}`]: () => new BN256G1Point(),
            [`${BN256G2Point.MARSHAL_ID.toString()}`]: () => new BN256G2Point(),
            [`${Ed25519Point.MARSHAL_ID.toString()}`]: () => new Ed25519Point(),
        };

        this.suites = {
            [`${PointFactory.SUITE_ED25519}`]: () => new Ed25519Point(),
            [`${PointFactory.SUITE_BN256}`]: () => new BN256G2Point(),
        };
    }

    /**
     * Decode a point using the 8 first bytes to look up the
     * correct instance
     * 
     * @param buf bytes to decode
     * @returns the point
     */
    public fromProto(buf: Buffer): Point {
        const tag = buf.slice(0, 8).toString();
        const generator = this.tags[tag];

        if (generator) {
            const point = generator();
            point.unmarshalBinary(buf.slice(8));

            return point
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
    public fromToml(suite: string, pub: string): Point {
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

export default new PointFactory();
