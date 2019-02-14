import BN = require('bn.js');
import { eddsa } from "elliptic";
import Curve from '../../../src/curve/edwards25519/curve';
import Point from '../../../src/curve/edwards25519/point';
import { PRNG } from '../../helpers/utils';

describe("Ed25519 Point", () => {
    const prng = new PRNG(42);
    const ec = new eddsa("ed25519");
    const curve = new Curve();
    // prettier-ignore
    const bytes = Buffer.from([5, 69, 248, 173, 171, 254, 19, 253, 143, 140, 146, 174, 26, 128, 3, 52, 106, 55, 112, 245, 62, 127, 42, 93, 0, 81, 47, 177, 30, 25, 39, 198]);

    beforeEach(() => prng.setSeed(42));

    it("should return the marshal data length", () => {
        expect(curve.point().marshalSize()).toBe(32);
    });

    it("should print the string representation of a point", () => {
        const point = curve.point();
        point.unmarshalBinary(bytes);

        expect(point.toString()).toBe(
            "0545f8adabfe13fd8f8c92ae1a8003346a3770f53e7f2a5d00512fb11e1927c6"
        );
    });

    it("should print the string representation of a null point", () => {
        const point = curve.point().null();

        expect(point.toString()).toBe(
            "0100000000000000000000000000000000000000000000000000000000000000"
        );
    });

    it("should retrieve the correct point", () => {
        const point = curve.point() as Point;
        point.unmarshalBinary(bytes);

        const targetX = new BN(
            "20279109212561463415328781515723337704810403260034987929787219898780302754141",
            10
        );
        const targetY = new BN(
            "4627191eb12f51005d2a7f3ef570376a3403801aae928c8ffd13feabadf84505",
            16
        );

        expect(point.ref.point.getX().cmp(targetX)).toBe(
            0,
            "X Coordinate unequal"
        );
        expect(point.ref.point.getY().cmp(targetY)).toBe(
            0,
            "Y Coordinate unequal"
        );
    });

    it("should work with a zero buffer", () => {
        const bytes = Buffer.alloc(curve.pointLen(), 0);
        const point = curve.point() as Point;
        point.unmarshalBinary(bytes);
        const target = new BN(
            "2b8324804fc1df0b2b4d00993dfbd7a72f431806ad2fe478c4ee1b274a0ea0b0",
            16
        );

        expect(point.ref.point.getX().cmp(target)).toBe(0);
    });

    it("should throw an exception on an invalid point", () => {
        prng.setSeed(42);
        const b = prng.pseudoRandomBytes(curve.pointLen());
        const point = curve.point();

        expect(() => point.unmarshalBinary(b)).toThrow();
    });

    it("should marshal the point according to spec", () => {
        let point = curve.point();
        point.unmarshalBinary(bytes);

        expect(point.marshalBinary()).toEqual(bytes);
    });

    it("should return true for equal points and false otherwise", () => {
        const x = new BN(
            "39139857753964535406422970543512609321558395110412588924902544519776250623903",
            10
        );
        const y = new BN(
            "22969022198784600445029639705880320580068058102470723316833310869386179114437",
            10
        );
        const a = new Point(x, y);
        const b = new Point(x, y);

        expect(a.equals(b)).toBeTruthy(
            "equals returns false for two equal points"
        );
        expect(a.equals(new Point())).toBeFalsy(
            "equal returns true for two unequal points"
        );
    });

    it("should set the point to the null element", () => {
        const point = curve.point().null() as Point;

        expect(point.ref.point.isInfinity()).toBeTruthy(
            "isInfinity returns false"
        );
    });

    it("should set the point to the base point", () => {
        const point = curve.point().base() as Point;
        const gx = new BN(
            "15112221349535400772501151409588531511454012693041857206046113283949847762202",
            10
        );
        const gy = new BN(
            "46316835694926478169428394003475163141307993866256225615783033603165251855960",
            10
        );

        expect(point.ref.point.x.fromRed().cmp(gx)).toBe(0, "x coord != gx");
        expect(point.ref.point.y.fromRed().cmp(gy)).toBe(0, "y coord != gy");
    });

    it("should pick a random point on the curve", () => {
        const point = curve.point().pick() as Point;

        expect(ec.curve.validate(point.ref.point)).toBeTruthy(
            "point not on curve"
        );
    });

    it("should pick a random point with a callback", () => {
        const point = curve.point().pick(prng.pseudoRandomBytes);
        const x = new BN(
            "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
            16
        );
        const y = new BN(
            "a56e25b707524601a885ebfba304e858a3fbf457a7a204b90fb9c2a888fbe467",
            16,
            "le"
        );
        const target = new Point(x, y);

        expect(point.equals(target)).toBeTruthy("point != target");
    });

    it("should point the receiver to another Point object", () => {
        const x = new BN(
            "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
            16
        );
        const y = new BN(
            "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
            16
        );
        const a = new Point(x, y);
        const b = curve.point().set(a) as Point;

        expect(a.equals(b)).toBeTruthy("a != b");
        a.base();
        expect(a.equals(b)).toBeTruthy("a != b");
    });

    it("should clone the point object", () => {
        const x = new BN(
            "365878b19a6b7603262993b9f1f05e20afc960a9addd9a8c89a4c59831d54bdd",
            16
        );
        const y = new BN(
            "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
            16
        );

        const a = new Point(x, y);
        const b = a.clone();

        expect(a.equals(b)).toBeTruthy();
        a.base();
        expect(a.equals(b)).toBeFalsy();
    });

    it("should return the embed length of point", () => {
        expect(curve.point().embedLen()).toBe(29, "Wrong embed length");
    });

    it("should throw an Error if data length > embedLen", () => {
        const point = curve.point();
        const data = Buffer.allocUnsafe(point.embedLen() + 1);

        expect(() => point.embed(data)).toThrow();
    });

    it("should embed data with length < embedLen", () => {
        const data = Buffer.from([1, 2, 3, 4, 5, 6]);
        const point = curve.point().embed(data, prng.pseudoRandomBytes);

        const x = new BN(
            "54122d7d6e2242d43785aa69043281bee5863075603b8a8346cb28f3b2a2e7ac",
            16
        );
        const y = new BN(
            "060102030405061c52e43010209ff7fed474ea3cf04f3fd69648912bc28c7819",
            16,
            "le"
        );
        const target = new Point(x, y);

        expect(point.equals(target)).toBeTruthy("point != target");
    });

    it("should embed data with length = embedLen", () => {
        // prettier-ignore
        const data = Buffer.from([68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73]);
        const point = curve.point().embed(data, prng.pseudoRandomBytes);

        const x = new BN(
            "7950d89da1c23ee557dd6e8fa59f12393366f2e9bdb4e69f880b44c272426bbf",
            16
        );
        const y = new BN(
            "441c49444544534944454453494445445349444544534944454453494445441d",
            16
        );

        const target = new Point(x, y);

        expect(point.equals(target)).toBeTruthy("point != target");
    });

    it("should extract embedded data", () => {
        const x = new BN(
            "54122d7d6e2242d43785aa69043281bee5863075603b8a8346cb28f3b2a2e7ac",
            16
        );
        const y = new BN(
            "19788cc22b914896d63f4ff03cea74d4fef79f201030e4521c06050403020106",
            16
        );
        const point = new Point(x, y);
        const data = Buffer.from([1, 2, 3, 4, 5, 6]);

        expect(point.data()).toEqual(data, "data returned wrong values");
    });

    it("should throw an Error on embeded length > embedLen", () => {
        const point = curve.point().base();

        expect(() => point.data()).toThrow();
    });

    it("should add two points", () => {
        prng.setSeed(42);

        const x3 = new BN(
            "57011823e7bf4d7bd56128c11a52c5410f50539ff235bc67599234eebae6689e",
            16
        );
        const y3 = new BN(
            "4d68a958f33a76d991fa87a9e6adc7187708bc6ad159e2fb23236669e5994e4a",
            16
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const p2 = curve.point().pick(prng.pseudoRandomBytes);
        const p3 = new Point(x3, y3);
        const sum = curve.point().add(p1, p2) as Point;
        // a + b = b + a
        const sum2 = curve.point().add(p2, p1) as Point;

        expect(ec.curve.validate(sum.ref.point)).toBeTruthy("sum not on curve");
        expect(sum.equals(p3)).toBeTruthy("sum != p3");
        expect(ec.curve.validate(sum2.ref.point)).toBeTruthy(
            "sum2 not on curve"
        );
        expect(sum2.equals(p3)).toBeTruthy("sum2 != p3");
    });

    it("should subtract two points", () => {
        prng.setSeed(42);

        const x3 = new BN(
            "680270df9cdcfdd959589470e041eecf05135db52ddd926135e250ae32fa68f9",
            16
        );
        const y3 = new BN(
            "49eef2fec1766d2e0d291d757e9e9882a6ae8ed74fa70e609e02aa09c446eb6e",
            16
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const p2 = curve.point().pick(prng.pseudoRandomBytes);
        const p3 = new Point(x3, y3);
        const diff = curve.point().sub(p1, p2) as Point;

        expect(ec.curve.validate(diff.ref.point)).toBeTruthy(
            "diff not on curve"
        );
        expect(diff.equals(p3)).toBeTruthy("diff != p3");
    });

    it("should negate a point", () => {
        prng.setSeed(42);
        const x2 = new BN(
            "49a7874e659489fcd9d66c460e0fa1df50369f5652226573765b3a67ce2ab410",
            16
        );
        const y2 = new BN(
            "67e4fb88a8c2b90fb904a2a757f4fba358e804a3fbeb85a801465207b7256ea5",
            16
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const p2 = new Point(x2, y2);
        const neg = curve.point().neg(p1) as Point;

        expect(ec.curve.validate(neg.ref.point)).toBeTruthy("neg not on curve");
        expect(neg.equals(p2)).toBeTruthy("neg != p2");
    });

    it("should negate null point", () => {
        const nullPoint = curve.point().null();
        const negNull = curve.point().neg(nullPoint);

        expect(negNull.equals(nullPoint)).toBeTruthy("negNull != nullPoint");
    });

    it("should multiply p by scalar s", () => {
        prng.setSeed(42);
        const x2 = new BN(
            "f570b26f180eeeb764e5a39b381b93ac44e2b7c3e93692440fe1a392ccdc5300",
            16,
            "le"
        );
        const y2 = new BN(
            "153780eef951eb9b02530195136fd27609e7d7c7c6b3e0820bdb121f44eec518",
            16,
            "le"
        );

        const p1 = curve.point().pick(prng.pseudoRandomBytes);
        const buf = Buffer.from([5, 10]);
        const s = curve.scalar().setBytes(buf);
        const prod = curve.point().mul(s, p1) as Point;
        const p2 = new Point(x2, y2);

        expect(ec.curve.validate(prod.ref.point)).toBeTruthy(
            "prod not on curve"
        );
        expect(prod.equals(p2)).toBeTruthy("prod != p2");
    });

    it("should multiply with base point if no point is passed", () => {
        const base = curve.point().base();
        const three = Buffer.from([3]);
        const threeScalar = curve.scalar().setBytes(three);
        const target = curve.point().mul(threeScalar, base) as Point;
        const threeBase = curve.point().mul(threeScalar) as Point;

        expect(ec.curve.validate(threeBase.ref.point)).toBeTruthy(
            "threeBase not on curve"
        );
        expect(threeBase.equals(target)).toBeTruthy("target != threeBase");
    });
});