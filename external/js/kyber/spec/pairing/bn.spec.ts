import BN from 'bn.js';
import { G1, G2, GT } from '../../src/pairing/bn';

describe('BN curve', () => {
    it('should add and multiply', () => {
        const p = new G1(123);

        const pp = new G1();
        pp.add(p, p);
        pp.add(pp, p);

        const pm = new G1();
        pm.scalarMul(p, new BN(3));

        expect(pp.equals(pm)).toBeTruthy();
    });

    it('should get the negative', () => {
        const p = new G1(123);

        const n = new G1();
        n.neg(p);

        const nn = new G1();
        nn.neg(n);

        expect(p.equals(n)).toBeFalsy();
        expect(nn.equals(p)).toBeTruthy();
    });

    it('should marshal a G1 point', () => {
        const v = '431168851f012abb1585d1f705a4988906211b6f19e30d22407ae5f6066f4dad8ad428edbf30349e340e5cf59deb08666b0c9280be656451c8a40aafb3223507';

        const p = new G1(111);
        const buf = p.marshal();

        expect(buf.toString('hex')).toBe(v);

        const pp = new G1();
        pp.unmarshal(buf);
        expect(pp.equals(p)).toBeTruthy();
    });

    it('should unmarshal the infinity for a G1 element', () => {
        const p = new G1();
        p.setInfinity();

        const buf = p.marshal();

        const p2 = new G1();
        p2.unmarshal(buf);

        expect(p2.isInfinity()).toBeTruthy();
    });

    it('should fail to unmarshal corrupted buffer or bad point', () => {
        const b1 = Buffer.from([1, 2, 3]);
        const b2 = Buffer.alloc(256 / 8 * 2, 1);
        const p = new G1();

        expect(() => p.unmarshal(b1)).toThrowError('wrong buffer size for a G1 point');
        expect(() => p.unmarshal(b2)).toThrowError('malformed G1 point');
    });

    it('should test the equality of a G1 element', () => {
        const p = new G1(123);
        const p2 = new G1(123);
        const p3 = new G1(12);

        expect(p.equals(p)).toBeTruthy();
        expect(p.equals(p2)).toBeTruthy();
        expect(p.equals(p3)).toBeFalsy();
        expect(p.equals(null)).toBeFalsy();
    });

    it('should stringify a G1 element', () => {
        const p = new G1();
        p.setInfinity();

        expect(p.toString()).toBe('bn256.G1(0,1)');
    });

    it('should add and multiply a G2 element', () => {
        const p = new G2(123);

        const pp = new G2();
        pp.add(p, p);
        pp.add(pp, p)

        const pm = new G2();
        pm.scalarMul(p, new BN(3));

        expect(pm.equals(pp)).toBeTruthy();
    });

    it('should get the negative of a G2 element', () => {
        const p = new G2(123);
        const n = new G2();
        n.neg(p);

        const nn = new G2();
        nn.neg(n);

        expect(p.equals(n)).toBeFalsy();
        expect(nn.equals(p)).toBeTruthy();
    });

    it('should marshal a G2 element', () => {
        const v = '21894d547009b7abecedfde89fd4fa82fe9d212d2b9f94a532e2ebfd360569fc4c65fa8eefac21f07c84d54407ec589281f36ba8c96d3114f3f3749d14f8ec0b6bca94f389776dde4597e402942cc184d82d37e81ed38046292c0f3522cf544a20a005ff2de92cf815fa5daa8defd6b064fda2adb1af2f10ee707aa996be98fa';

        const p = new G2(111);
        const buf = p.marshal();

        expect(buf.toString('hex')).toBe(v);

        const pp = new G2();
        pp.unmarshal(buf);
        expect(pp.equals(p)).toBeTruthy();
    });

    it('should marshal the infinity', () => {
        const p = new G2();
        p.setInfinity();

        const buf = p.marshal();

        const p2 = new G2();
        p2.unmarshal(buf);

        expect(p2.isInfinity()).toBeTruthy();
    });

    it('should fail to unmarshal a corrupted buffer or a bad point', () => {
        const b1 = Buffer.from([1, 2, 3]);
        const b2 = Buffer.alloc(256 / 8 * 4, 1);
        const p = new G2();

        expect(() => p.unmarshal(b1)).toThrowError('wrong buffer size for G2 point');
        expect(() => p.unmarshal(b2)).toThrowError('malformed G2 point');
    });

    it('should test the equality of a G2 element', () => {
        const p = new G2(123);
        const p2 = p.clone();
        const p3 = new BN(new BN(12));

        expect(p.equals(p)).toBeTruthy();
        expect(p.equals(p2)).toBeTruthy();
        expect(p.equals(p3)).toBeFalsy();
        expect(p.equals(null));
    });

    it('should stringify a G2 element', () => {
        const p = new G2();
        p.setInfinity();

        expect(p.toString()).toBe('bn256.G2((0,0),(0,1),(0,0))');
    });

    it('should marshal a GT element', () => {
        const p = GT.one();
        p.scalarMul(p, new BN(123456789));

        const buf = p.marshal();
        const pp = new GT();
        pp.unmarshal(buf);

        expect(pp.equals(p)).toBeTruthy();
    });

    it('should fail to unmarshal a corrupted buffer', () => {
        const buf = Buffer.from([1, 2, 3]);
        const p = new GT();

        expect(() => p.unmarshal(buf)).toThrow();
    });

    it('should test the equality of a GT element', () => {
        const g1 = new G1(new BN(12345));
        const g2 = new G2(new BN(67890));

        const p = GT.pair(g1, g2);
        const p2 = GT.pair(g1, g2);

        const p3 = GT.one();

        expect(p.equals(p2)).toBeTruthy();
        expect(p.equals(p)).toBeTruthy();
        expect(p.equals(p3)).toBeFalsy();
        expect(p.equals(null)).toBeFalsy();
    });

    it('should stringify a GT element', () => {
        const p = GT.one();

        expect(p.toString()).toBe('bn256.GT(((0,0), (0,0), (0,0)), ((0,0), (0,0), (0,1)))');
    });

    it('should go through a tripartite DH protocol', () => {
        const a = new BN(123);
        const b = new BN(456);
        const c = new BN(789);

        const pa = new G1();
        pa.scalarBaseMul(a);
        const qa = new G2();
        qa.scalarBaseMul(a);
        const pb = new G1();
        pb.scalarBaseMul(b);
        const qb = new G2();
        qb.scalarBaseMul(b);
        const pc = new G1();
        pc.scalarBaseMul(c);
        const qc = new G2();
        qc.scalarBaseMul(c);

        const k1 = GT.pair(pb, qc);
        k1.scalarMul(k1, a);
        const k1Bytes = k1.marshal();
        const kk1 = new GT();
        kk1.unmarshal(k1Bytes);
        expect(k1Bytes).toEqual(kk1.marshal());

        const k2 = GT.pair(pc, qa);
        k2.scalarMul(k2, b);
        const k2Bytes = k2.marshal();
        const kk2 = new GT();
        kk2.unmarshal(k2Bytes);
        expect(k2Bytes).toEqual(kk2.marshal());

        const k3 = GT.pair(pa, qb);
        k3.scalarMul(k3, c);
        const k3Bytes = k3.marshal();
        const kk3 = new GT();
        kk3.unmarshal(k3Bytes);
        expect(k3Bytes).toEqual(kk3.marshal());

        expect(k1Bytes).toEqual(k2Bytes);
        expect(k1Bytes).toEqual(k3Bytes);
    });
});
