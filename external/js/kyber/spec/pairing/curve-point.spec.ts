import BN from 'bn.js';
import CurvePoint from '../../src/pairing/curve-point';
import { order } from '../../src/pairing/constants';

describe('BN256 Curve Point', () => {
    it('should add one', () => {
        const one = new CurvePoint();
        one.mul(CurvePoint.generator, new BN(1));

        const g = new CurvePoint();
        g.mul(CurvePoint.generator, order);
        expect(g.isInfinity()).toBeTruthy();

        g.add(g, one);
        g.makeAffine();

        expect(g.equals(one)).toBeTruthy();
        expect(g.isOnCurve()).toBeTruthy();
        expect(one.isOnCurve()).toBeTruthy();
    });

    it('should add and double', () => {
        const a = new CurvePoint();
        a.mul(CurvePoint.generator, new BN(123456789));

        const aa = new CurvePoint();
        aa.add(a, a);

        const d = new CurvePoint();
        d.dbl(a);

        expect(aa.getX().equals(d.getX())).toBeTruthy();
        expect(aa.getY().equals(d.getY())).toBeTruthy();
    });

    it('should add infinity', () => {
        const inf = new CurvePoint();
        inf.setInfinity();
        expect(inf.isInfinity()).toBeTruthy();

        const one = new CurvePoint();
        one.mul(CurvePoint.generator, new BN(1));

        const t = new CurvePoint();
        t.add(inf, one);
        expect(t.getX().equals(one.getX())).toBeTruthy();
        expect(t.getY().equals(one.getY())).toBeTruthy();

        t.add(one, inf);
        expect(t.getX().equals(one.getX())).toBeTruthy();
        expect(t.getY().equals(one.getY())).toBeTruthy();
    });

    it('should make basic operations', () => {
        const g = new CurvePoint();
        g.copy(CurvePoint.generator);

        const x = new BN('32498273234');
        const X = new CurvePoint()
        X.mul(g, x);

        const y = new BN('98732423523');
        const Y = new CurvePoint()
        Y.mul(g, y);

        const s1 = new CurvePoint()
        s1.mul(X, y);
        s1.makeAffine();

        const s2 = new CurvePoint();
        s2.mul(Y, x);
        s2.makeAffine();

        expect(s1.getX().compareTo(s2.getX())).toBe(0);
        expect(s2.getX().compareTo(s1.getX())).toBe(0);
    });

    it('should negate the point', () => {
        const p = new CurvePoint();
        p.mul(CurvePoint.generator, new BN(12345));

        const np = new CurvePoint();
        np.negative(p);

        expect(p.getY().equals(np.getY())).toBeFalsy();

        const nnp = new CurvePoint();
        nnp.negative(np);
        expect(p.getX().equals(nnp.getX())).toBeTruthy();
        expect(p.getY().equals(nnp.getY())).toBeTruthy();
    });

    it('should make the point affine', () => {
        const p = new CurvePoint();
        p.makeAffine();
        expect(p.isInfinity()).toBeTruthy();
    });

    it('should test the equality', () => {
        const p = new CurvePoint();
        p.mul(CurvePoint.generator, new BN(123));

        const p2 = new CurvePoint();
        p2.mul(CurvePoint.generator, new BN(123));

        const p3 = new CurvePoint();
        p3.mul(CurvePoint.generator, new BN(12));

        expect(p.equals(p)).toBeTruthy();
        expect(p.equals(p2)).toBeTruthy();
        expect(p3.equals(p)).toBeFalsy();
        expect(p.equals(null)).toBeFalsy();
    });

    it('should stringify', () => {
        const p = new CurvePoint();
        p.setInfinity();

        expect(p.toString()).toBe('(0,1)');
    });
});
