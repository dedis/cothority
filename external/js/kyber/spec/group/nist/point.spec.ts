import BN = require('bn.js');
import Nist from '../../../src/curve/nist';
import NistPoint from '../../../src/curve/nist/point';
import { PRNG } from '../../helpers/utils';

const { Curve, Params, Point } = Nist;

describe('Nist Point', () => {
    const prng = new PRNG(42);
    const curve = new Curve(Params.p256);
    // prettier-ignore
    const bytes = Buffer.from([4, 86, 136, 95, 219, 46, 44, 21, 129, 228, 251, 109, 189, 14, 233, 39, 200, 61, 230, 250, 80, 93, 166, 150, 168, 69, 80, 207, 81, 252, 111, 247, 159, 50, 200, 1, 128, 38, 107, 124, 61, 43, 130, 195, 203, 232, 91, 139, 88, 232, 48, 229, 188, 47, 236, 127, 85, 30, 29, 69, 170, 227, 163, 245, 197]);

    beforeEach(() => {
        prng.setSeed(42);
    });

    it("should return the marshal data length", () => {
        expect(curve.point().marshalSize()).toBe(65);
    });

    it("should print the string representation of a point", () => {
        const point = curve.point();

        // prettier-ignore
        const target = "(39139857753964535406422970543512609321558395110412588924902544519776250623903,22969022198784600445029639705880320580068058102470723316833310869386179114437)";
        point.unmarshalBinary(bytes);

        expect(point.toString()).toBe(target);
    });

    it("should print the string representation of a null point", () => {
        const point = curve.point().null();
        const target = "(0,0)";

        expect(point.toString()).toBe(target);
    });

    it("should retrieve the correct point", () => {
        const point = new Point(curve);
        point.unmarshalBinary(bytes);

        const targetX = new BN(
            "39139857753964535406422970543512609321558395110412588924902544519776250623903",
            10
        );
        const targetY = new BN(
            "22969022198784600445029639705880320580068058102470723316833310869386179114437",
            10
        );

        expect(point.ref.point.x.fromRed().cmp(targetX)).toBe(0);
        expect(point.ref.point.y.fromRed().cmp(targetY)).toBe(0);
    });

    it("should work with a zero buffer", () => {
        const bytes = Buffer.alloc(curve.pointLen(), 0);
        bytes[0] = 4;
        const point = new Point(curve);
        point.unmarshalBinary(bytes);

        expect(point.ref.point.isInfinity());
    });

    it("should throw an exception on an invalid point", () => {
        let b = Buffer.from(bytes);
        b[1] = 11;
        expect(() => curve.point().unmarshalBinary(b)).toThrow();

        b = Buffer.from(bytes);
        b[0] = 5;
        expect(() => curve.point().unmarshalBinary(b)).toThrow();
    });

    it("should marshal the point according to spec", () => {
        const point = curve.point();
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
        const a = new Point(curve, x, y);
        const b = new Point(curve, x, y);
        expect(a.equals(b)).toBeTruthy();
        expect(a.equals(new Point(curve))).toBeFalsy();
    });

    it("should set the point to the null element", () => {
        const point = new Point(curve).null();

        expect(point.ref.point.x).toBeNull();
        expect(point.ref.point.y).toBeNull();
        expect(point.ref.point.isInfinity());
    });

    it("should set the point to the base point", () => {
        const point = new Point(curve).base();
        const gx = new BN(
            "48439561293906451759052585252797914202762949526041747995844080717082404635286",
            10
        );
        const gy = new BN(
            "36134250956749795798585127919587881956611106672985015071877198253568414405109",
            10
        );

        expect(point.ref.point.x.fromRed().cmp(gx)).toBe(0);
        expect(point.ref.point.y.fromRed().cmp(gy)).toBe(0);
    });


    it("should pick a random point on the curve", () => {
        const point = new Point(curve).pick();

        expect(curve.curve.validate(point.ref.point)).toBeTruthy();
    });

    it("should pick a random point with a callback", () => {
        const point = new Point(curve).pick(prng.pseudoRandomBytes);
        const x = new BN(
            "20748411802786496192214829937999303328497424855966679862814184095607022057120",
            10
        );
        const y = new BN(
            "25265734854388630912039078646990097162089342591470146115346718827076733979605",
            10
        );
        const target = new Point(curve, x, y);

        expect(point.equals(target)).toBeTruthy();
    });

    it("should point the receiver to another Point object", () => {
        const x = new BN(
            "110886497124999652792301595882074601258209437127400215444890406848187894132914",
            10
        );

        const y = new BN(
            "53575054073404339774035073443964261709325086507923884144188368935145165925208",
            10
        );
        const a = new Point(curve, x, y);
        const b = curve.point().set(a) as NistPoint;

        expect(a.equals(b)).toBeTruthy("a != b");
        a.base();
        expect(a.equals(b)).toBeTruthy("a != b");
    });

    it("should clone the point object", () => {
        const x = new BN(
            "110886497124999652792301595882074601258209437127400215444890406848187894132914",
            10
        );

        const y = new BN(
            "53575054073404339774035073443964261709325086507923884144188368935145165925208",
            10
        );
        const a = new Point(curve, x, y);
        const b = a.clone();

        expect(a.equals(b)).toBeTruthy("a != b");
        a.base();
        expect(a.equals(b)).toBeFalsy("a == b");
    });

    it("should return the embed length of point", () => {
        expect(curve.point().embedLen()).toBe(30);
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
            "102139584446102187259397119781836631801548481932383950198227714785215701648902",
            10
        );
        const y = new BN(
            "5861343223324868218838501643187874518624053854803548931377838939237611808222",
            10
        );
        const target = new Point(curve, x, y);

        expect(point.equals(target)).toBeTruthy("point != target");
    });

    it("should embed data with length = embedLen", () => {
        // prettier-ignore
        const data = Buffer.from([68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83, 68, 69, 68, 73, 83]);
        const point = curve.point().embed(data, prng.pseudoRandomBytes);

        const x = new BN(
            "55302791189059782649052548129238946009757254481983766602279814276765761229598",
            10
        );
        const y = new BN(
            "61392340999017759760595650359991412638601309952067335192619881551703613872215",
            10
        );

        const target = new Point(curve, x, y);
        expect(point.equals(target)).toBeTruthy("point != target");
    });

    it("should extract embedded data", () => {
        const x = new BN(
            "73817101515731741206419437436780703770216083351543551172201261211469822298629",
            10
        );
        const y = new BN(
            "88765585354250601676518938452147013066867582066920092774664806337343312248839",
            10
        );
        const point = new Point(curve, x, y);
        const data = Buffer.from([2, 4, 6, 8, 10]);

        expect(point.data()).toEqual(data);
    });

    it("should throw an Error on embeded length > embedLen", () => {
        prng.pseudoRandomBytes(65);
        // prettier-ignore
        const bytes = Buffer.from([4, 201, 209, 147, 190, 134, 219, 80, 165, 6, 231, 153, 126, 240, 204, 175, 212, 170, 3, 0, 156, 228, 220, 14, 189, 212, 105, 250, 84, 26, 5, 195, 137, 6, 162, 237, 154, 18, 5, 159, 120, 82, 140, 135, 94, 18, 162, 95, 112, 39, 108, 199, 167, 17, 65, 78, 9, 156, 173, 246, 10, 104, 224, 192, 157]);
        const point = curve.point();
        point.unmarshalBinary(bytes);

        expect(() => point.data()).toThrow();
    });

    it("should add two points", () => {
        const x1 = new BN(
            "110886497124999652792301595882074601258209437127400215444890406848187894132914",
            10
        );
        const y1 = new BN(
            "53575054073404339774035073443964261709325086507923884144188368935145165925208",
            10
        );

        const x2 = new BN(
            "34573993922878482166947855247172961010057054898826823466355875611648896895957",
            10
        );
        const y2 = new BN(
            "80433135225576650318672668154277196065185839134887563174194682903173809692971",
            10
        );

        const x3 = new BN(
            "29544735125058142535927177955671190517757885143989883513559208755530587788814",
            10
        );
        const y3 = new BN(
            "87130155067619906103393670168292422617898997176123455498428905788640774614716",
            10
        );

        const p1 = new Point(curve, x1, y1);
        const p2 = new Point(curve, x2, y2);
        const p3 = new Point(curve, x3, y3);
        const sum = curve.point().add(p1, p2) as NistPoint;
        // a + b = b + a
        const sum2 = curve.point().add(p2, p1) as NistPoint;

        expect(curve.curve.validate(sum.ref.point)).toBeTruthy();
        expect(sum.equals(p3)).toBeTruthy();
        expect(curve.curve.validate(sum2.ref.point)).toBeTruthy();
        expect(sum2.equals(p3)).toBeTruthy();
    });

    it("should subtract two points", () => {
        const x1 = new BN(
            "110886497124999652792301595882074601258209437127400215444890406848187894132914",
            10
        );
        const y1 = new BN(
            "53575054073404339774035073443964261709325086507923884144188368935145165925208",
            10
        );

        const x2 = new BN(
            "34573993922878482166947855247172961010057054898826823466355875611648896895957",
            10
        );
        const y2 = new BN(
            "80433135225576650318672668154277196065185839134887563174194682903173809692971",
            10
        );

        const x3 = new BN(
            "110976464347423563140926045017315078040251245233675452604569570242275790207423",
            10
        );
        const y3 = new BN(
            "27383322053734574189591792613880900856836445243349135319283759784129102955550",
            10
        );

        const p1 = new Point(curve, x1, y1);
        const p2 = new Point(curve, x2, y2);
        const p3 = new Point(curve, x3, y3);
        const diff = curve.point().sub(p1, p2) as NistPoint;

        expect(curve.curve.validate(diff.ref.point)).toBeTruthy();
        expect(diff.equals(p3)).toBeTruthy();
    });

    it("should negate a point", () => {
        const x1 = new BN(
            "110886497124999652792301595882074601258209437127400215444890406848187894132914",
            10
        );
        const y1 = new BN(
            "53575054073404339774035073443964261709325086507923884144188368935145165925208",
            10
        );

        const x2 = new BN(
            "110886497124999652792301595882074601258209437127400215444890406848187894132914",
            10
        );
        const y2 = new BN(
            "62217035136951908988662373505443311820761056907366430051345262373721931928743",
            10
        );

        const p1 = new Point(curve, x1, y1);
        const p2 = new Point(curve, x2, y2);
        const neg = curve.point().neg(p1) as NistPoint;

        expect(curve.curve.validate(neg.ref.point)).toBeTruthy();
        expect(neg.equals(p2)).toBeTruthy();
    });

    it("should negate null point", () => {
        const nullPoint = curve.point().null();
        const negNull = curve.point().neg(nullPoint);

        expect(negNull.equals(nullPoint)).toBeTruthy();
    });

    describe("mul", () => {
        it("should multiply p by scalar s", () => {
            const x1 = new BN(
                "110886497124999652792301595882074601258209437127400215444890406848187894132914",
                10
            );
            const y1 = new BN(
                "53575054073404339774035073443964261709325086507923884144188368935145165925208",
                10
            );
            const x2 = new BN(
                "1597317178653134404256011203581619582648667990919174345469188685075376970978",
                10
            );
            const y2 = new BN(
                "35697504603312581621426847792434067144582647671367433296669068730814336637643",
                10
            );

            const p1 = new Point(curve, x1, y1);
            const buf = Buffer.from([5, 10]);
            const s = curve.scalar().setBytes(buf);
            const prod = curve.point().mul(s, p1) as NistPoint;
            const p2 = new Point(curve, x2, y2);

            expect(curve.curve.validate(prod.ref.point)).toBeTruthy();
            expect(prod.equals(p2)).toBeTruthy();
        });

        it("should multiply with base point if no point is passed", () => {
            const base = curve.point().base();
            const three = Buffer.from([3]);
            const threeScalar = curve.scalar().setBytes(three);
            const target = curve.point().mul(threeScalar, base) as NistPoint;
            const threeBase = curve.point().mul(threeScalar) as NistPoint;

            expect(curve.curve.validate(threeBase.ref.point)).toBeTruthy();
            expect(threeBase.equals(target)).toBeTruthy();
        });
    });
});
