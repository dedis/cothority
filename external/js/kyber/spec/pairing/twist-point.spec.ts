import BN from 'bn.js';
import TwistPoint from '../../src/pairing/twist-point';
import { order } from '../../src/pairing/constants';

describe('Twist Point', () => {
    it('should add one', () => {
        const g = new TwistPoint();
        g.mul(TwistPoint.generator, order);
        expect(g.isInfinity()).toBeTruthy();

        const one = new TwistPoint();
        one.mul(TwistPoint.generator, new BN(1));
        
        g.add(g, one);
        g.makeAffine();

        expect(g.getX().equals(one.getX())).toBeTruthy();
        expect(g.isOnCurve()).toBeTruthy();
    });

    it('should negate the point', () => {
        const p = new TwistPoint();
        p.mul(TwistPoint.generator, new BN(3));

        const n = new TwistPoint();
        n.neg(p);

        expect(n.getX().equals(p.getX())).toBeTruthy();
        expect(n.getY().equals(p.getY())).toBeFalsy();

        const nn = new TwistPoint();
        nn.neg(n);

        expect(nn.getX().equals(p.getX())).toBeTruthy();
        expect(nn.getY().equals(p.getY())).toBeTruthy();
    });

    it('should add and multiply', () => {
        const p = new TwistPoint();
        p.mul(TwistPoint.generator, new BN(123456789));

        const d = new TwistPoint();
        d.add(p, p);
        d.makeAffine();

        const m = new TwistPoint();
        m.mul(p, new BN(2));
        m.makeAffine();

        expect(d.getX().equals(m.getX())).toBeTruthy();
        expect(d.getY().equals(m.getY())).toBeTruthy();
    });

    it('should add the infinity', () => {
        const p = new TwistPoint();
        p.mul(TwistPoint.generator, new BN(123));

        const inf = new TwistPoint();
        inf.setInfinity();
        expect(inf.isOnCurve()).toBeTruthy();

        const t = new TwistPoint();
        t.add(p, inf);
        expect(t.getX().equals(p.getX())).toBeTruthy();
        expect(t.getY().equals(p.getY())).toBeTruthy();

        t.add(inf, p);
        expect(t.getX().equals(p.getX())).toBeTruthy();
        expect(t.getY().equals(p.getY())).toBeTruthy();
    });

    it('should test the equality', () => {
        const p = new TwistPoint();
        p.mul(TwistPoint.generator, new BN(123));

        const p2 = new TwistPoint();
        p2.mul(TwistPoint.generator, new BN(123));

        const p3 = new TwistPoint();
        p3.mul(TwistPoint.generator, new BN(12));

        expect(p.equals(p)).toBeTruthy();
        expect(p.equals(p2)).toBeTruthy();
        expect(p.equals(p3)).toBeFalsy();
        expect(p.equals(null)).toBeFalsy();
    });

    it('should stringify', () => {
        const inf = new TwistPoint();
        inf.setInfinity();

        expect(inf.toString()).toBe('((0,0),(0,1),(0,0))');
    });
});
