import PointFactory from '../src/point-factory';
import { curve } from '../src';
import { BN256G1Point, BN256G2Point } from '../src/pairing/point';

const ed25519 = curve.newCurve('edwards25519');

describe('Point Factory Tests', () => {
    it('should decode a point from protobuf', () => {
        expect(() => PointFactory.fromProto(Buffer.from([]))).toThrow();

        // Edwards25519
        for (let i = 0; i < 100; i++) {
            const p = ed25519.point().pick();
            const buf = p.toProto();

            const p2 = PointFactory.fromProto(buf);
            expect(p.equals(p2)).toBeTruthy();
        }

        // BN256 G1
        for (let i = 0; i < 100; i++) {
            const p = new BN256G1Point().pick();
            const buf = p.toProto();

            const p2 = PointFactory.fromProto(buf);
            expect(p.equals(p2)).toBeTruthy();
        }

        // BN256 G2
        for (let i = 0; i < 100; i++) {
            const p = new BN256G2Point().pick();
            const buf = p.toProto();

            const p2 = PointFactory.fromProto(buf);
            expect(p.equals(p2)).toBeTruthy();
        }
    });

    it('should decode a point from TOML', () => {
        expect(() => PointFactory.fromToml('', 'deadbeef')).toThrow();

        // Edwards25519
        for (let i = 0; i < 100; i++) {
            const p = ed25519.point().pick();
            const buf = p.marshalBinary().toString('hex');

            const p2 = PointFactory.fromToml('Ed25519', buf);
            expect(p.equals(p2)).toBeTruthy();
        }

        // BN256 G2
        for (let i = 0; i < 100; i++) {
            const p = new BN256G2Point().pick();
            const buf = p.marshalBinary().toString('hex');

            const p2 = PointFactory.fromToml('bn256.adapter', buf);
            expect(p.equals(p2)).toBeTruthy();
        }
    });
});
